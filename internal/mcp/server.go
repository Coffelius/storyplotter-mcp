package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
)

// UserStore abstracts per-user StoryPlotter data persistence. The concrete
// disk-backed implementation lives in the data package; see GAB-93.
type UserStore interface {
	Load(userID string) (*data.Export, error)
	Save(userID string, exp *data.Export) error
	Raw(userID string) ([]byte, error)
	Replace(userID string, raw []byte) error
}

// CallContext carries per-request identity and storage access to tool handlers.
type CallContext struct {
	Ctx       context.Context
	UserID    string // "" means anon / shared fallback.
	Store     UserStore
	Signer    *TokenSigner // nil in stdio mode (download is HTTP-only).
	PublicURL string       // base URL used to build /download links; "" falls back in-tool.
}

// ToolHandler is the function a tool registers.
type ToolHandler func(args json.RawMessage, cc *CallContext) (CallToolResult, error)

// Tool bundles a definition with its handler.
type Tool struct {
	Def     ToolDefinition
	Handler ToolHandler
}

// Server holds the registered tools and the user-aware data store.
type Server struct {
	Store     UserStore
	Signer    *TokenSigner // optional; required only for request_export_link / /download.
	PublicURL string       // optional; used to build absolute download links.
	tools     map[string]Tool
	mu        sync.RWMutex
	limiter   *Limiter
}

// NewServer returns a new Server backed by the given UserStore. Signer and
// publicURL may be nil/empty in stdio mode; HTTP mode wires them in from
// env vars (see cmd/storyplotter-mcp/main.go).
func NewServer(store UserStore, signer *TokenSigner, publicURL string) *Server {
	return &Server{
		Store:     store,
		Signer:    signer,
		PublicURL: publicURL,
		tools:     map[string]Tool{},
	}
}

// Register adds a tool.
func (s *Server) Register(t Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[t.Def.Name] = t
}

// toolList returns a copy of the registered tool definitions.
func (s *Server) toolList() []ToolDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ToolDefinition, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t.Def)
	}
	return out
}

// Dispatch handles one request. Accepts the originating *http.Request so the
// user-context middleware can surface identity to tool handlers. Pass r == nil
// for stdio / in-process callers — in that case identity falls back to the
// shared corpus (UserID "").
func (s *Server) Dispatch(r *http.Request, req *Request) *Response {
	// Notifications (no id) don't get a response.
	isNotification := len(req.ID) == 0 || string(req.ID) == "null"

	switch req.Method {
	case "initialize":
		return s.ok(req.ID, InitializeResult{
			ProtocolVersion: ProtocolVersion,
			Capabilities:    Capabilities{Tools: map[string]any{}},
			ServerInfo:      ServerInfo{Name: ServerName, Version: ServerVersion},
		})
	case "initialized", "notifications/initialized":
		return nil
	case "ping":
		return s.ok(req.ID, map[string]any{})
	case "tools/list":
		return s.ok(req.ID, ToolsListResult{Tools: s.toolList()})
	case "tools/call":
		return s.handleToolsCall(r, req)
	default:
		if isNotification {
			return nil
		}
		return s.err(req.ID, CodeMethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
	}
}

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleToolsCall(r *http.Request, req *Request) *Response {
	var p callParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return s.err(req.ID, CodeInvalidParams, "invalid params: "+err.Error())
		}
	}
	s.mu.RLock()
	t, ok := s.tools[p.Name]
	s.mu.RUnlock()
	if !ok {
		return s.err(req.ID, CodeMethodNotFound, "unknown tool: "+p.Name)
	}
	cc := s.callContext(r)
	res, err := t.Handler(p.Arguments, cc)
	if err != nil {
		return s.ok(req.ID, ErrorResult(err.Error()))
	}
	return s.ok(req.ID, res)
}

// callContext builds a CallContext from the request (nil for stdio).
func (s *Server) callContext(r *http.Request) *CallContext {
	if r == nil {
		return &CallContext{
			Ctx:       context.Background(),
			UserID:    "",
			Store:     s.Store,
			Signer:    s.Signer,
			PublicURL: s.PublicURL,
		}
	}
	return &CallContext{
		Ctx:       r.Context(),
		UserID:    UserIDFromContext(r.Context()),
		Store:     s.Store,
		Signer:    s.Signer,
		PublicURL: s.PublicURL,
	}
}

func (s *Server) ok(id json.RawMessage, result any) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Result: result}
}

func (s *Server) err(id json.RawMessage, code int, msg string) *Response {
	return &Response{JSONRPC: "2.0", ID: id, Error: &RPCError{Code: code, Message: msg}}
}

// ServeStdio runs a newline-delimited JSON-RPC loop on the given reader/writer.
func (s *Server) ServeStdio(r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	enc := json.NewEncoder(w)
	for {
		var req Request
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			// Parse error: send an error response with null id and continue.
			resp := &Response{JSONRPC: "2.0", Error: &RPCError{Code: CodeParseError, Message: err.Error()}}
			_ = enc.Encode(resp)
			return err
		}
		resp := s.Dispatch(nil, &req)
		if resp == nil {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
}
