package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/subagent"
)

// TaskInput defines the input for the Task tool.
type TaskInput struct {
	AgentName string `json:"agent_name" jsonschema:"required,description=Name of the sub-agent to spawn"`
	Prompt    string `json:"prompt" jsonschema:"required,description=Task description for the sub-agent"`
	MaxTurns  *int   `json:"max_turns,omitempty" jsonschema:"description=Override max turns for this run"`
}

// TaskTool spawns a sub-agent via the Runner and blocks until it completes.
type TaskTool struct {
	runner *subagent.Runner
}

// NewTaskTool creates a TaskTool backed by the given Runner.
func NewTaskTool(runner *subagent.Runner) *TaskTool {
	return &TaskTool{runner: runner}
}

var _ agent.Tool[TaskInput] = (*TaskTool)(nil)

func (t *TaskTool) Name() string        { return "Task" }
func (t *TaskTool) Description() string { return "Spawn a sub-agent to perform a task and return its result" }

func (t *TaskTool) Execute(ctx context.Context, input TaskInput) (*agent.ToolResult, error) {
	if input.AgentName == "" {
		return agent.ErrorResult("agent_name is required"), nil
	}
	if input.Prompt == "" {
		return agent.ErrorResult("prompt is required"), nil
	}

	runID, err := t.runner.Spawn(ctx, input.AgentName, input.Prompt)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to spawn sub-agent: %s", err.Error())), nil
	}

	result, err := t.runner.Wait(ctx, runID)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("sub-agent wait failed: %s", err.Error())), nil
	}

	if result.Err != nil {
		return agent.ErrorResult(fmt.Sprintf("sub-agent error: %s", result.Err.Error())), nil
	}

	if result.Output == "" {
		return agent.TextResult("(sub-agent completed with no output)"), nil
	}

	return agent.TextResult(result.Output), nil
}
