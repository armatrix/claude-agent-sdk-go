package mcp

import "errors"

// Sentinel errors for the MCP package.
var (
	// ErrNotConnected is returned when attempting to use a transport that
	// has not yet established a connection.
	ErrNotConnected = errors.New("mcp: server not connected")

	// ErrServerNotFound is returned when referencing a server name that
	// does not exist in the Manager.
	ErrServerNotFound = errors.New("mcp: server not found")

	// ErrToolNotFound is returned when a bridged tool name cannot be
	// resolved to a known server/tool pair.
	ErrToolNotFound = errors.New("mcp: tool not found")

	// ErrInvalidConfig is returned when a ServerConfig is missing
	// required fields for its transport type.
	ErrInvalidConfig = errors.New("mcp: invalid server config")
)
