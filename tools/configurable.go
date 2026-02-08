package tools

import agent "github.com/armatrix/claude-agent-sdk-go"

// BuiltinOptions holds callbacks for configurable built-in tools.
type BuiltinOptions struct {
	AskCallback  AskCallback
	PlanCallback PlanCallback
}

// RegisterConfigurable registers built-in tools that require callbacks.
// Call this in addition to RegisterAll for the interactive tools.
func RegisterConfigurable(registry *agent.ToolRegistry, opts BuiltinOptions) {
	if opts.AskCallback != nil {
		agent.RegisterTool(registry, &AskTool{Callback: opts.AskCallback})
	}
	// TodoTool doesn't need a callback, always register it.
	agent.RegisterTool(registry, &TodoTool{})

	if opts.PlanCallback != nil {
		agent.RegisterTool(registry, &PlanModeTool{Callback: opts.PlanCallback})
	}
}
