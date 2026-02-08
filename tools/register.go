package tools

import (
	agent "github.com/armatrix/claude-agent-sdk-go"
)

// RegisterAll registers all built-in tools into the provided registry.
func RegisterAll(registry *agent.ToolRegistry) {
	agent.RegisterTool(registry, &ReadTool{})
	agent.RegisterTool(registry, &WriteTool{})
	agent.RegisterTool(registry, &EditTool{})
	agent.RegisterTool(registry, &BashTool{})
	agent.RegisterTool(registry, &GlobTool{})
	agent.RegisterTool(registry, &GrepTool{})
}
