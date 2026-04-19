package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPConfig configures the HTTP transport.
type HTTPConfig struct {
	Addr   string
	Bearer string // required if non-empty
}

// Handler returns an http.Handler that serves /mcp and /healthz.
func (s *Server) Handler(cfg HTTPConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthz)
	mux.Handle("/mcp", bearerMiddleware(cfg.Bearer, http.HandlerFunc(s.serveMCP)))
	return mux
}

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
	resp := s.Dispatch(&req)

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
