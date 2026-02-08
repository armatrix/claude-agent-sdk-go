package subagent

import (
	"context"
	"encoding/json"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/internal/schema"
)

// WithDefinitions returns an AgentOption that registers sub-agent definitions
// and the Task tool. The Runner is created during Agent initialization.
//
// Usage:
//
//	a := agent.NewAgent(
//	    subagent.WithDefinitions(
//	        subagent.Definition{Name: "researcher", Instructions: "..."},
//	        subagent.Definition{Name: "coder", Model: anthropic.ModelClaudeHaiku4_5},
//	    ),
//	)
func WithDefinitions(defs ...Definition) agent.AgentOption {
	return agent.WithOnInit(func(a *agent.Agent) {
		if len(defs) == 0 {
			return
		}
		defMap := make(map[string]*Definition, len(defs))
		for i := range defs {
			defMap[defs[i].Name] = &defs[i]
		}
		runner := NewRunner(a, defMap)
		registerTaskTool(a.Tools(), runner)
	})
}

// taskInput defines the input for the auto-registered Task tool.
// This mirrors tools.TaskInput to keep schemas in sync while avoiding
// an import cycle (tools -> subagent -> tools).
type taskInput struct {
	AgentName string `json:"agent_name" jsonschema:"required,description=Name of the sub-agent to spawn"`
	Prompt    string `json:"prompt" jsonschema:"required,description=Task description for the sub-agent"`
	MaxTurns  *int   `json:"max_turns,omitempty" jsonschema:"description=Override max turns for this run"`
}

// registerTaskTool registers the Task tool directly into the registry using
// RegisterRaw, avoiding import of the tools package (which imports subagent,
// creating a cycle).
func registerTaskTool(registry *agent.ToolRegistry, runner *Runner) {
	s := schema.Generate[taskInput]()
	registry.RegisterRaw(
		"Task",
		"Spawn a sub-agent to perform a task and return its result",
		s,
		func(ctx context.Context, raw json.RawMessage) (*agent.ToolResult, error) {
			var input taskInput
			if err := json.Unmarshal(raw, &input); err != nil {
				return agent.ErrorResult(fmt.Sprintf("invalid input: %s", err)), nil
			}
			if input.AgentName == "" {
				return agent.ErrorResult("agent_name is required"), nil
			}
			if input.Prompt == "" {
				return agent.ErrorResult("prompt is required"), nil
			}

			runID, err := runner.Spawn(ctx, input.AgentName, input.Prompt)
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("failed to spawn sub-agent: %s", err)), nil
			}

			result, err := runner.Wait(ctx, runID)
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("sub-agent wait failed: %s", err)), nil
			}

			if result.Err != nil {
				return agent.ErrorResult(fmt.Sprintf("sub-agent error: %s", result.Err)), nil
			}

			if result.Output == "" {
				return agent.TextResult("(sub-agent completed with no output)"), nil
			}

			return agent.TextResult(result.Output), nil
		},
	)
}
