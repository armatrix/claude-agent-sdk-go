package mcp

import (
	"context"
	"encoding/json"
)

// ToolInfo describes a tool discovered from an MCP server.
type ToolInfo struct {
	// Name is the tool's name as reported by the server.
	Name string

	// Description is a human-readable description of the tool.
	Description string

	// InputSchema is the raw JSON schema for the tool's input.
	InputSchema json.RawMessage
}

// Transport is the interface for communicating with an MCP server.
// Implementations handle the underlying protocol (stdio, HTTP/SSE, etc.).
type Transport interface {
	// Connect establishes the connection to the MCP server.
	Connect(ctx context.Context) error

	// ListTools discovers available tools from the server.
	ListTools(ctx context.Context) ([]ToolInfo, error)

	// CallTool invokes a tool on the server by name with the given arguments.
	CallTool(ctx context.Context, name string, args map[string]any) (string, error)

	// ListResources discovers available resources from the server.
	ListResources(ctx context.Context) ([]Resource, error)

	// ReadResource reads a resource by URI from the server.
	ReadResource(ctx context.Context, uri string) (string, error)

	// Close tears down the connection and releases resources.
	Close() error
}

// NewTransport creates a Transport for the given ServerConfig based on its
// Transport type. Returns ErrInvalidConfig if the config is not valid.
func NewTransport(cfg ServerConfig) (Transport, error) {
	switch cfg.Transport {
	case TransportStdio:
		return NewStdioTransport(cfg)
	case TransportSSE, TransportStreamableHTTP:
		return NewHTTPTransport(cfg)
	default:
		// Default to stdio if command is set, HTTP if URL is set.
		if cfg.Command != "" {
			return NewStdioTransport(cfg)
		}
		if cfg.URL != "" {
			return NewHTTPTransport(cfg)
		}
		return nil, ErrInvalidConfig
	}
}
