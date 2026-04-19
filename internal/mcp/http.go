package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// HTTPConfig configures the HTTP transport.
type HTTPConfig struct {
	Addr   string
	Bearer string // required if non-empty

	// BodyLimit caps request body size in bytes. <=0 means no cap.
	BodyLimit int64
	// MCPRateLimitPerMin caps /mcp calls per key per minute. <=0 disables.
	MCPRateLimitPerMin int
	// DownloadRateLimitPerMin caps /download calls per key per minute.
	// Consumed by DownloadMiddleware once the /download route lands.
	DownloadRateLimitPerMin int
}

// contextKey is a typed key for request-scoped values.
type contextKey string

const userIDContextKey contextKey = "storyplotter.user_id"

// UserIDHeader is the HTTP header LibreChat populates (via its dynamic
// placeholder {{LIBRECHAT_USER_ID}}) per-request.
const UserIDHeader = "X-LibreChat-User-Id"

// userIDPattern matches LibreChat user ids (ObjectId hex or similar). We
// deliberately accept letters, digits, underscore, and hyphen — 1..64 chars.
var userIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// UserIDFromContext returns the user id stashed by userContextMiddleware, or "".
func UserIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(userIDContextKey).(string); ok {
		return v
	}
	return ""
}

// Handler returns an http.Handler that serves /mcp, /healthz and /download.
//
// - /mcp is bearer-protected and resolves user identity from the
//   X-LibreChat-User-Id header.
// - /healthz is bearer-free (probed by Uptime Kuma / Coolify healthchecks).
// - /download is bearer-free — the signed token in ?t=... is the auth.
func (s *Server) Handler(cfg HTTPConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)

	limiter := NewLimiter()
	s.limiter = limiter

	// /mcp: bearer + user-id context + rate limit + body cap.
	mcpCore := bearerMiddleware(cfg.Bearer, userContextMiddleware(http.HandlerFunc(s.serveMCP)))
	mcpWrapped := http.Handler(mcpCore)
	if cfg.MCPRateLimitPerMin > 0 {
		mcpWrapped = WithRateLimit(mcpWrapped, limiter, cfg.MCPRateLimitPerMin, time.Minute, keyFnMCP)
	}
	if cfg.BodyLimit > 0 {
		mcpWrapped = WithBodyLimit(mcpWrapped, cfg.BodyLimit)
	}
	mux.Handle("/mcp", mcpWrapped)

	// /download: signed token is the auth (no Bearer). IP-based rate limit —
	// the /download route never sees the LibreChat user header, so keyFnMCP
	// naturally falls back to ip:<client ip>.
	var dlHandler http.Handler = http.HandlerFunc(s.serveDownload)
	if cfg.DownloadRateLimitPerMin > 0 {
		dlHandler = DownloadMiddleware(limiter, cfg.DownloadRateLimitPerMin, dlHandler)
	}
	mux.Handle("/download", dlHandler)
	return mux
}

// Limiter exposes the shared rate limiter built for this server's HTTP
// transport. Returns nil if Handler has not been called. Used by the Wave 2A
// /download integration to share a single bucket namespace across routes.
func (s *Server) Limiter() *Limiter { return s.limiter }

// ServeHTTP starts the HTTP listener on cfg.Addr (blocking).
func (s *Server) ServeHTTP(cfg HTTPConfig) error {
	if cfg.Bearer == "" {
		return fmt.Errorf("MCP_BEARER is required for HTTP mode")
	}
	return http.ListenAndServe(cfg.Addr, s.Handler(cfg))
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func bearerMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token == "" {
			http.Error(w, "server misconfigured: missing bearer", http.StatusInternalServerError)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// userContextMiddleware extracts the optional X-LibreChat-User-Id header and
// injects a validated user id into the request context. Missing header is
// not an error (falls back to shared corpus); malformed header -> 400.
func userContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := strings.TrimSpace(r.Header.Get(UserIDHeader))
		if uid != "" && !userIDPattern.MatchString(uid) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid X-LibreChat-User-Id"}`))
			return
		}
		ctx := context.WithValue(r.Context(), userIDContextKey, uid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) serveMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req Request
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	resp := s.Dispatch(r, &req)

	// Stream via SSE for LibreChat compatibility.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	if resp != nil {
		b, _ := json.Marshal(resp)
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	// Closing frame for SSE consumers that look for it.
	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}
