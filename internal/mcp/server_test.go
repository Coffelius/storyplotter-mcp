package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
)

// memStore is a minimal in-memory UserStore for tests.
type memStore struct {
	shared *data.Export
	users  map[string]*data.Export
}

func (m *memStore) Load(userID string) (*data.Export, error) {
	if userID == "" {
		if m.shared == nil {
			return &data.Export{}, nil
		}
		return m.shared, nil
	}
	if e, ok := m.users[userID]; ok {
		return e, nil
	}
	return &data.Export{}, nil
}

func (m *memStore) Save(userID string, exp *data.Export) error {
	if userID == "" {
		return nil
	}
	if m.users == nil {
		m.users = map[string]*data.Export{}
	}
	m.users[userID] = exp
	return nil
}

func (m *memStore) Raw(string) ([]byte, error)       { return nil, nil }
func (m *memStore) Replace(string, []byte) error     { return nil }

func newTestServer() *Server {
	return NewServer(&memStore{})
}

func TestInitialize(t *testing.T) {
	s := newTestServer()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	var out bytes.Buffer
	// ServeStdio will block until EOF, which happens after the single request.
	done := make(chan error, 1)
	go func() { done <- s.ServeStdio(in, &out) }()
	err := <-done
	if err != nil && err != io.EOF {
		t.Fatalf("serve: %v", err)
	}
	var resp Response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v; body=%s", err, out.String())
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc = %q", resp.JSONRPC)
	}
	b, _ := json.Marshal(resp.Result)
	var got InitializeResult
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("result: %v", err)
	}
	if got.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocolVersion = %q", got.ProtocolVersion)
	}
	if got.ServerInfo.Name != ServerName {
		t.Errorf("serverInfo.name = %q", got.ServerInfo.Name)
	}
}

func TestToolsListEmpty(t *testing.T) {
	s := newTestServer()
	resp := s.Dispatch(nil, &Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "tools/list"})
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp: %+v", resp)
	}
	b, _ := json.Marshal(resp.Result)
	var got ToolsListResult
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(got.Tools))
	}
}

func TestMethodNotFound(t *testing.T) {
	s := newTestServer()
	resp := s.Dispatch(nil, &Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "bogus"})
	if resp.Error == nil || resp.Error.Code != CodeMethodNotFound {
		t.Errorf("expected method not found, got %+v", resp)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	s := newTestServer()
	resp := s.Dispatch(nil, &Request{JSONRPC: "2.0", Method: "initialized"})
	if resp != nil {
		t.Errorf("expected nil response for notification, got %+v", resp)
	}
}

// --- user-context middleware tests ---

func TestUserContextHeaderValid(t *testing.T) {
	var seen string
	h := userContextMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(UserIDHeader, "abc123_user")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if seen != "abc123_user" {
		t.Errorf("UserID = %q, want %q", seen, "abc123_user")
	}
}

func TestUserContextHeaderAbsent(t *testing.T) {
	var seen = "sentinel"
	h := userContextMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	if seen != "" {
		t.Errorf("UserID = %q, want empty", seen)
	}
}

func TestUserContextHeaderInvalid(t *testing.T) {
	h := userContextMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatalf("inner handler should not run")
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set(UserIDHeader, "hello world") // space not allowed
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "invalid X-LibreChat-User-Id") {
		t.Errorf("body = %s", rr.Body.String())
	}
}

func TestUserIDFromContextNil(t *testing.T) {
	if got := UserIDFromContext(nil); got != "" {
		t.Errorf("want empty, got %q", got)
	}
	if got := UserIDFromContext(context.Background()); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}
