package mcp

import (
	"context"
	"fmt"
)

// StdioTransport implements the Transport interface for subprocess-based MCP
// servers. It communicates via the subprocess's stdin/stdout using JSON-RPC.
//
// Currently this is a stub — Connect validates the config and stores the
// command info, but does not actually spawn a subprocess. Full JSON-RPC
// framing will be implemented in a future phase.
type StdioTransport struct {
	command   string
	args      []string
	env       map[string]string
	connected bool
}

var _ Transport = (*StdioTransport)(nil)

// NewStdioTransport creates a new StdioTransport from the given config.
// Returns ErrInvalidConfig if Command is empty.
func NewStdioTransport(cfg ServerConfig) (*StdioTransport, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("%w: stdio transport requires command", ErrInvalidConfig)
	}
	return &StdioTransport{
		command: cfg.Command,
		args:    cfg.Args,
		env:     cfg.Env,
	}, nil
}

// Connect validates the configuration and marks the transport as ready.
// In a future phase, this will spawn the subprocess and perform the MCP
// handshake.
func (t *StdioTransport) Connect(_ context.Context) error {
	// Stub: mark as connected for now.
	// Real implementation will: exec.CommandContext, start, JSON-RPC initialize.
	t.connected = true
	return nil
}

// ListTools returns ErrNotConnected since the subprocess is not yet spawned.
func (t *StdioTransport) ListTools(_ context.Context) ([]ToolInfo, error) {
	if !t.connected {
		return nil, ErrNotConnected
	}
	// Stub: real implementation will send JSON-RPC tools/list request.
	return nil, nil
}

// CallTool returns ErrNotConnected since the subprocess is not yet spawned.
func (t *StdioTransport) CallTool(_ context.Context, _ string, _ map[string]any) (string, error) {
	if !t.connected {
		return "", ErrNotConnected
	}
	// Stub: real implementation will send JSON-RPC tools/call request.
	return "", fmt.Errorf("%w: stdio call not yet implemented", ErrNotConnected)
}

// ListResources returns ErrNotConnected since the subprocess is not yet spawned.
func (t *StdioTransport) ListResources(_ context.Context) ([]Resource, error) {
	if !t.connected {
		return nil, ErrNotConnected
	}
	// Stub: real implementation will send JSON-RPC resources/list request.
	return nil, nil
}

// ReadResource returns ErrNotConnected since the subprocess is not yet spawned.
func (t *StdioTransport) ReadResource(_ context.Context, _ string) (string, error) {
	if !t.connected {
		return "", ErrNotConnected
	}
	// Stub: real implementation will send JSON-RPC resources/read request.
	return "", fmt.Errorf("%w: stdio read not yet implemented", ErrNotConnected)
}

// Close terminates the subprocess if running.
func (t *StdioTransport) Close() error {
	t.connected = false
	// Stub: real implementation will kill the subprocess and close pipes.
	return nil
}
