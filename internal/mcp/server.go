package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/Coffelius/storyplotter-mcp/internal/data"
)

// ToolHandler is the function a tool registers.
type ToolHandler func(args json.RawMessage, d *data.Export) (CallToolResult, error)

// Tool bundles a definition with its handler.
type Tool struct {
	Def     ToolDefinition
	Handler ToolHandler
}

// Server holds the registered tools and a reference to the loaded data.
type Server struct {
	Data  *data.Export
	tools map[string]Tool
	mu    sync.RWMutex
}

// NewServer returns a new Server.
func NewServer(d *data.Export) *Server {
	return &Server{Data: d, tools: map[string]Tool{}}
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

// Dispatch handles one request. Returns nil response for notifications.
func (s *Server) Dispatch(req *Request) *Response {
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
		return s.handleToolsCall(req)
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

func (s *Server) handleToolsCall(req *Request) *Response {
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
	res, err := t.Handler(p.Arguments, s.Data)
	if err != nil {
		return s.ok(req.ID, ErrorResult(err.Error()))
	}
	return s.ok(req.ID, res)
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
		resp := s.Dispatch(&req)
		if resp == nil {
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
}
