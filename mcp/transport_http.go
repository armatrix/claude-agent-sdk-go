package mcp

import (
	"context"
	"fmt"
)

// HTTPTransport implements the Transport interface for HTTP-based MCP servers
// (both SSE and streamable-http transport types).
//
// Currently this is a stub — Connect validates the config and stores the URL,
// but does not actually establish an HTTP connection. Full HTTP/SSE support
// will be implemented in a future phase.
type HTTPTransport struct {
	url           string
	transportType TransportType
	connected     bool
}

var _ Transport = (*HTTPTransport)(nil)

// NewHTTPTransport creates a new HTTPTransport from the given config.
// Returns ErrInvalidConfig if URL is empty.
func NewHTTPTransport(cfg ServerConfig) (*HTTPTransport, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("%w: HTTP transport requires URL", ErrInvalidConfig)
	}
	tt := cfg.Transport
	if tt == "" {
		tt = TransportSSE
	}
	return &HTTPTransport{
		url:           cfg.URL,
		transportType: tt,
	}, nil
}

// Connect validates the configuration and marks the transport as ready.
// In a future phase, this will establish the SSE/HTTP connection.
func (t *HTTPTransport) Connect(_ context.Context) error {
	// Stub: mark as connected for now.
	// Real implementation will: HTTP client setup, SSE stream, handshake.
	t.connected = true
	return nil
}

// ListTools returns an empty list since HTTP is not yet connected.
func (t *HTTPTransport) ListTools(_ context.Context) ([]ToolInfo, error) {
	if !t.connected {
		return nil, ErrNotConnected
	}
	// Stub: real implementation will send HTTP tools/list request.
	return nil, nil
}

// CallTool returns an error since HTTP is not yet fully implemented.
func (t *HTTPTransport) CallTool(_ context.Context, _ string, _ map[string]any) (string, error) {
	if !t.connected {
		return "", ErrNotConnected
	}
	// Stub: real implementation will send HTTP tools/call request.
	return "", fmt.Errorf("%w: HTTP call not yet implemented", ErrNotConnected)
}

// ListResources returns an empty list since HTTP is not yet connected.
func (t *HTTPTransport) ListResources(_ context.Context) ([]Resource, error) {
	if !t.connected {
		return nil, ErrNotConnected
	}
	// Stub: real implementation will send HTTP resources/list request.
	return nil, nil
}

// ReadResource returns an error since HTTP is not yet fully implemented.
func (t *HTTPTransport) ReadResource(_ context.Context, _ string) (string, error) {
	if !t.connected {
		return "", ErrNotConnected
	}
	// Stub: real implementation will send HTTP resources/read request.
	return "", fmt.Errorf("%w: HTTP read not yet implemented", ErrNotConnected)
}

// Close tears down the HTTP connection.
func (t *HTTPTransport) Close() error {
	t.connected = false
	// Stub: real implementation will close HTTP client and SSE stream.
	return nil
}
