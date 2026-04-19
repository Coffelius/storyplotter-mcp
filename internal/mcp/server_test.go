package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/gabistuff/storyplotter-mcp/internal/data"
)

func newTestServer() *Server {
	return NewServer(&data.Export{})
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
	resp := s.Dispatch(&Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "tools/list"})
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
	resp := s.Dispatch(&Request{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "bogus"})
	if resp.Error == nil || resp.Error.Code != CodeMethodNotFound {
		t.Errorf("expected method not found, got %+v", resp)
	}
}

func TestNotificationNoResponse(t *testing.T) {
	s := newTestServer()
	resp := s.Dispatch(&Request{JSONRPC: "2.0", Method: "initialized"})
	if resp != nil {
		t.Errorf("expected nil response for notification, got %+v", resp)
	}
}
