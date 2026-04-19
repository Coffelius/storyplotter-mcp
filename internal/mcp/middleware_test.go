package mcp

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func passthrough() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
	})
}

func TestWithBodyLimit_Exceeds(t *testing.T) {
	h := WithBodyLimit(passthrough(), 100)
	body := bytes.Repeat([]byte("x"), 200)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rr.Code)
	}
}

func TestWithBodyLimit_UnderCap(t *testing.T) {
	h := WithBodyLimit(passthrough(), 100)
	body := bytes.Repeat([]byte("x"), 50)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if rr.Body.Len() != 50 {
		t.Errorf("echoed body len = %d, want 50", rr.Body.Len())
	}
}

func TestWithRateLimit_BlocksAfterMax(t *testing.T) {
	l := NewLimiter()
	keyFn := func(r *http.Request) string { return r.Header.Get("X-Key") }
	h := WithRateLimit(passthrough(), l, 60, time.Minute, keyFn)

	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
		req.Header.Set("X-Key", "same")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200", i+1, rr.Code)
		}
	}

	// 61st denied.
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	req.Header.Set("X-Key", "same")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("61st: status = %d, want 429", rr.Code)
	}
	if ra := rr.Header().Get("Retry-After"); ra == "" {
		t.Errorf("missing Retry-After header")
	}

	// Different key still allowed.
	req2 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	req2.Header.Set("X-Key", "other")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Errorf("different key: status = %d, want 200", rr2.Code)
	}
}

func TestKeyFnMCP(t *testing.T) {
	// With valid user id header.
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(UserIDHeader, "user_42")
	if got := keyFnMCP(req); got != "uid:user_42" {
		t.Errorf("keyFnMCP = %q, want uid:user_42", got)
	}

	// No header -> ip.
	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	req2.RemoteAddr = "10.0.0.5:1234"
	if got := keyFnMCP(req2); got != "ip:10.0.0.5" {
		t.Errorf("keyFnMCP = %q, want ip:10.0.0.5", got)
	}

	// X-Forwarded-For preferred.
	req3 := httptest.NewRequest(http.MethodPost, "/", nil)
	req3.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req3.RemoteAddr = "10.0.0.5:1234"
	if got := keyFnMCP(req3); got != "ip:1.2.3.4" {
		t.Errorf("keyFnMCP = %q, want ip:1.2.3.4", got)
	}
}
