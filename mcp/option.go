package mcp

import (
	"context"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// WithServers returns an AgentOption that connects to MCP servers and
// registers their tools into the agent's ToolRegistry.
//
// MCP servers are connected eagerly during Agent construction so tools are
// available before the first Run(). Connection errors are non-fatal — servers
// that fail to connect are silently skipped. Use the manual NewManager +
// RegisterBridgedTools path if you need explicit error handling.
//
// The Agent.Close() method will disconnect all MCP servers.
//
// Usage:
//
//	a := agent.NewAgent(
//	    mcp.WithServers(map[string]mcp.ServerConfig{
//	        "context7": {Command: "npx", Args: []string{"@context7/mcp"}, Transport: mcp.TransportStdio},
//	    }),
//	)
//	defer a.Close()
func WithServers(servers map[string]ServerConfig) agent.AgentOption {
	return agent.WithOnInit(func(a *agent.Agent) {
		if len(servers) == 0 {
			return
		}
		mgr := NewManager(servers)

		// Best-effort connect — errors are non-fatal (tools just won't appear).
		_ = mgr.Connect(context.Background())

		RegisterBridgedTools(a.Tools(), mgr)

		// Register cleanup so Agent.Close() disconnects MCP servers.
		a.AddCleanup(mgr.Close)
	})
}

// WithTransports returns an AgentOption that connects pre-built MCP transports
// and registers their tools. This is primarily useful for testing with mock
// transports.
//
// Unlike WithServers, this uses pre-injected transports rather than creating
// them from ServerConfig.
func WithTransports(transports map[string]Transport) agent.AgentOption {
	return agent.WithOnInit(func(a *agent.Agent) {
		if len(transports) == 0 {
			return
		}
		mgr := NewManagerWithTransports(transports)

		// Best-effort connect.
		_ = mgr.ConnectWithTransports(context.Background())

		RegisterBridgedTools(a.Tools(), mgr)

		a.AddCleanup(mgr.Close)
	})
}
