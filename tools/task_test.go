package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/subagent"
)

// --- Helper: create a runner with a mock run function ---

func newTestRunner(defs map[string]*subagent.Definition, fn subagent.RunFunc) *subagent.Runner {
	parent := agent.NewAgent()
	return subagent.NewRunnerWithRunFunc(parent, defs, fn)
}

// --- TaskTool basic tests ---

func TestTaskTool_Name(t *testing.T) {
	tool := NewTaskTool(nil)
	assert.Equal(t, "Task", tool.Name())
}

func TestTaskTool_Description(t *testing.T) {
	tool := NewTaskTool(nil)
	assert.NotEmpty(t, tool.Description())
}

// --- TaskTool input validation ---

func TestTaskTool_Execute_EmptyAgentName(t *testing.T) {
	runner := newTestRunner(
		map[string]*subagent.Definition{},
		func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
			return &subagent.Result{Output: "unused"}
		},
	)
	tool := NewTaskTool(runner)

	result, err := tool.Execute(context.Background(), TaskInput{
		AgentName: "",
		Prompt:    "do something",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	text := extractText(result)
	assert.Contains(t, text, "agent_name is required")
}

func TestTaskTool_Execute_EmptyPrompt(t *testing.T) {
	runner := newTestRunner(
		map[string]*subagent.Definition{},
		func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
			return &subagent.Result{Output: "unused"}
		},
	)
	tool := NewTaskTool(runner)

	result, err := tool.Execute(context.Background(), TaskInput{
		AgentName: "worker",
		Prompt:    "",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	text := extractText(result)
	assert.Contains(t, text, "prompt is required")
}

// --- TaskTool spawn failure ---

func TestTaskTool_Execute_DefinitionNotFound(t *testing.T) {
	runner := newTestRunner(
		map[string]*subagent.Definition{},
		func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
			return &subagent.Result{Output: "unused"}
		},
	)
	tool := NewTaskTool(runner)

	result, err := tool.Execute(context.Background(), TaskInput{
		AgentName: "nonexistent",
		Prompt:    "do something",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	text := extractText(result)
	assert.Contains(t, text, "failed to spawn sub-agent")
	assert.Contains(t, text, "definition not found")
}

// --- TaskTool successful execution ---

func TestTaskTool_Execute_Success(t *testing.T) {
	defs := map[string]*subagent.Definition{
		"worker": {Name: "worker"},
	}
	runner := newTestRunner(defs, func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
		return &subagent.Result{
			Output: "task completed: " + prompt,
		}
	})
	tool := NewTaskTool(runner)

	result, err := tool.Execute(context.Background(), TaskInput{
		AgentName: "worker",
		Prompt:    "analyze the data",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	text := extractText(result)
	assert.Equal(t, "task completed: analyze the data", text)
}

// --- TaskTool with sub-agent error ---

func TestTaskTool_Execute_SubagentError(t *testing.T) {
	defs := map[string]*subagent.Definition{
		"worker": {Name: "worker"},
	}
	runner := newTestRunner(defs, func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
		return &subagent.Result{
			Output: "partial output",
			Err:    errors.New("internal failure"),
		}
	})
	tool := NewTaskTool(runner)

	result, err := tool.Execute(context.Background(), TaskInput{
		AgentName: "worker",
		Prompt:    "do something risky",
	})

	require.NoError(t, err)
	assert.True(t, result.IsError)
	text := extractText(result)
	assert.Contains(t, text, "sub-agent error")
	assert.Contains(t, text, "internal failure")
}

// --- TaskTool with empty output ---

func TestTaskTool_Execute_EmptyOutput(t *testing.T) {
	defs := map[string]*subagent.Definition{
		"worker": {Name: "worker"},
	}
	runner := newTestRunner(defs, func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
		return &subagent.Result{Output: ""}
	})
	tool := NewTaskTool(runner)

	result, err := tool.Execute(context.Background(), TaskInput{
		AgentName: "worker",
		Prompt:    "generate something",
	})

	require.NoError(t, err)
	assert.False(t, result.IsError)
	text := extractText(result)
	assert.Contains(t, text, "no output")
}

// --- TaskTool registration ---

func TestTaskTool_RegisterTool(t *testing.T) {
	defs := map[string]*subagent.Definition{
		"worker": {Name: "worker"},
	}
	runner := newTestRunner(defs, func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
		return &subagent.Result{Output: "ok"}
	})

	registry := agent.NewToolRegistry()
	agent.RegisterTool(registry, NewTaskTool(runner))

	names := registry.Names()
	assert.Contains(t, names, "Task")

	tools := registry.ListForAPI()
	require.Len(t, tools, 1)
	assert.Equal(t, "Task", tools[0].OfTool.Name)
}

// --- TaskTool context cancellation ---

func TestTaskTool_Execute_ContextCancelled(t *testing.T) {
	defs := map[string]*subagent.Definition{
		"worker": {Name: "worker"},
	}
	runner := newTestRunner(defs, func(ctx context.Context, a *agent.Agent, prompt string) *subagent.Result {
		<-ctx.Done()
		return &subagent.Result{
			Output: "cancelled",
			Err:    ctx.Err(),
		}
	})
	tool := NewTaskTool(runner)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	result, err := tool.Execute(ctx, TaskInput{
		AgentName: "worker",
		Prompt:    "slow task",
	})

	require.NoError(t, err)
	// The result should indicate an error since the context was cancelled.
	assert.True(t, result.IsError)
}
