// Package mcp implements a minimal Model Context Protocol server
// over stdio and HTTP. JSON-RPC 2.0 messages are exchanged as
// newline-delimited JSON over stdio (per the MCP stdio transport spec).
package mcp

import "encoding/json"

const ProtocolVersion = "2024-11-05"
const ServerName = "storyplotter-mcp"
const ServerVersion = "0.1.0"

// Request is a JSON-RPC 2.0 request or notification.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError is a JSON-RPC 2.0 error object.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// InitializeResult is returned from `initialize`.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools map[string]any `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolDefinition is an entry in the tools/list response.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult is returned from `tools/list`.
type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// Content is one content block in a tools/call result.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallToolResult is returned from `tools/call`.
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// TextResult is a convenience constructor.
func TextResult(text string) CallToolResult {
	return CallToolResult{Content: []Content{{Type: "text", Text: text}}}
}

// ErrorResult wraps an error message as a tools/call result.
func ErrorResult(msg string) CallToolResult {
	return CallToolResult{Content: []Content{{Type: "text", Text: msg}}, IsError: true}
}
