package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabistuff/storyplotter-mcp/internal/data"
)

func TestHTTPInitializeWithBearer(t *testing.T) {
	s := NewServer(&data.Export{})
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content-type = %q", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	raw := string(b)
	if !strings.Contains(raw, "event: message") {
		t.Errorf("missing SSE message event: %s", raw)
	}
	// extract the data line json
	dataLine := ""
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			break
		}
	}
	if dataLine == "" {
		t.Fatalf("no data line in: %s", raw)
	}
	var rpcResp Response
	if err := json.Unmarshal([]byte(dataLine), &rpcResp); err != nil {
		t.Fatalf("unmarshal: %v; data=%s", err, dataLine)
	}
	rb, _ := json.Marshal(rpcResp.Result)
	var ir InitializeResult
	if err := json.Unmarshal(rb, &ir); err != nil {
		t.Fatal(err)
	}
	if ir.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocolVersion = %q", ir.ProtocolVersion)
	}
	if ir.ServerInfo.Name != ServerName {
		t.Errorf("server name = %q", ir.ServerInfo.Name)
	}
}

func TestHTTPRejectsWithoutBearer(t *testing.T) {
	s := NewServer(&data.Export{})
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`)
	resp, err := http.Post(ts.URL+"/mcp", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	s := NewServer(&data.Export{})
	ts := httptest.NewServer(s.Handler(HTTPConfig{Bearer: "secret"}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"ok"`) {
		t.Errorf("body = %s", b)
	}
}
