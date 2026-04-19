package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

// serveDownload handles GET /download?t=<token>. The token is minted by the
// request_export_link tool; verification consumes it one-shot. This route
// intentionally bypasses Bearer middleware: the signed token IS the auth.
func (s *Server) serveDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Signer == nil {
		http.Error(w, "download not configured on this server", http.StatusServiceUnavailable)
		return
	}
	token := r.URL.Query().Get("t")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}
	userID, err := s.Signer.Verify(token)
	if err != nil {
		switch {
		case errors.Is(err, ErrTokenBadSignature), errors.Is(err, ErrTokenMalformed):
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case errors.Is(err, ErrTokenExpired), errors.Is(err, ErrTokenReused):
			http.Error(w, "link no longer valid", http.StatusGone)
		default:
			http.Error(w, "token verification failed", http.StatusInternalServerError)
		}
		return
	}
	raw, err := s.Store.Raw(userID)
	if err != nil || len(raw) == 0 {
		http.Error(w, "no data for this account", http.StatusNotFound)
		return
	}
	filename := fmt.Sprintf("storyplotter-%s.json", time.Now().UTC().Format("20060102-1504"))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
