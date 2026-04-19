package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strings"
	"time"
)

// WithBodyLimit returns an http.Handler that caps the request body to maxBytes.
// On overflow the handler responds 413 without invoking next. Otherwise the
// body is buffered in-memory and passed through to next as an io.NopCloser.
func WithBodyLimit(next http.Handler, maxBytes int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || maxBytes <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		limited := http.MaxBytesReader(w, r.Body, maxBytes)
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, limited); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				_, _ = w.Write([]byte(`{"error":"request body too large"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, `{"error":"read body: %s"}`, err.Error())
			return
		}
		_ = r.Body.Close()
		r.Body = io.NopCloser(&buf)
		r.ContentLength = int64(buf.Len())
		next.ServeHTTP(w, r)
	})
}

// WithRateLimit returns an http.Handler that enforces max events per window
// on the bucket returned by keyFn(r). On deny it responds 429 with a
// Retry-After header (whole seconds, ceil).
func WithRateLimit(next http.Handler, l *Limiter, max int, window time.Duration, keyFn func(r *http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if l == nil || max <= 0 || window <= 0 {
			next.ServeHTTP(w, r)
			return
		}
		key := keyFn(r)
		allowed, retry := l.Allow(key, max, window)
		if !allowed {
			secs := int(math.Ceil(retry.Seconds()))
			if secs < 1 {
				secs = 1
			}
			w.Header().Set("Retry-After", fmt.Sprintf("%d", secs))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts a best-effort client IP from r. It prefers the first
// comma-separated entry of X-Forwarded-For, falling back to r.RemoteAddr
// with any port stripped.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.IndexByte(xff, ','); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// keyFnMCP returns the rate-limit bucket key for MCP traffic: uid:<id> when
// the LibreChat user header is present and valid, otherwise ip:<client ip>.
func keyFnMCP(r *http.Request) string {
	uid := strings.TrimSpace(r.Header.Get(UserIDHeader))
	if uid != "" && userIDPattern.MatchString(uid) {
		return "uid:" + uid
	}
	return "ip:" + clientIP(r)
}

// DownloadMiddleware wraps a handler intended for the /download route with
// the same body limit + per-user rate limit stack as /mcp. It exists so the
// Wave 2A branch can compose middleware at merge time without re-deriving
// keyFn / limiter wiring.
func DownloadMiddleware(limiter *Limiter, perMin int, next http.Handler) http.Handler {
	return WithRateLimit(next, limiter, perMin, time.Minute, keyFnMCP)
}
