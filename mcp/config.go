// Package mcp provides an MCP (Model Context Protocol) client for connecting
// to external tool servers. It bridges remote MCP tools into the agent's
// local ToolRegistry so the agent loop can call them transparently.
package mcp

// TransportType identifies the MCP transport protocol.
type TransportType string

const (
	// TransportStdio communicates via a subprocess's stdin/stdout.
	TransportStdio TransportType = "stdio"

	// TransportSSE communicates via HTTP Server-Sent Events.
	TransportSSE TransportType = "sse"

	// TransportStreamableHTTP communicates via HTTP streaming.
	TransportStreamableHTTP TransportType = "streamable-http"
)

// ServerConfig describes how to connect to a single MCP server.
type ServerConfig struct {
	// Command is the executable to spawn (stdio transport only).
	Command string

	// Args are command-line arguments for the subprocess.
	Args []string

	// Env are extra environment variables for the subprocess.
	Env map[string]string

	// URL is the server address (SSE and streamable-http transports).
	URL string

	// Transport selects the communication protocol.
	Transport TransportType
}
