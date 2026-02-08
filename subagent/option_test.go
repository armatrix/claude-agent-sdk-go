package subagent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

func TestWithDefinitions_RegistersTaskTool(t *testing.T) {
	a := agent.NewAgent(
		WithDefinitions(
			Definition{Name: "researcher", Instructions: "Research things"},
			Definition{Name: "coder", Model: anthropic.ModelClaudeHaiku4_5},
		),
	)

	names := a.Tools().Names()
	assert.Contains(t, names, "Task")
}

func TestWithDefinitions_Empty_NoTaskTool(t *testing.T) {
	a := agent.NewAgent(WithDefinitions())

	names := a.Tools().Names()
	assert.NotContains(t, names, "Task")
}

func TestWithDefinitions_TaskToolSchema(t *testing.T) {
	a := agent.NewAgent(
		WithDefinitions(
			Definition{Name: "worker"},
		),
	)

	tools := a.Tools().ListForAPI()
	require.Len(t, tools, 1)
	assert.Equal(t, "Task", tools[0].OfTool.Name)
	assert.Contains(t, tools[0].OfTool.Description.Value, "sub-agent")
}

func TestWithDefinitions_TaskToolExecute_Success(t *testing.T) {
	// Use a custom RunFunc to avoid real API calls.
	var capturedAgent *agent.Agent

	a := agent.NewAgent(
		agent.WithOnInit(func(ag *agent.Agent) {
			capturedAgent = ag
		}),
		// We need to use WithOnInit + manual registration since
		// WithDefinitions uses defaultRunFunc. Instead, test via
		// a Runner with a mock RunFunc.
	)

	// Create runner with echo mock.
	defs := map[string]*Definition{
		"worker": {Name: "worker"},
	}
	runner := NewRunnerWithRunFunc(a, defs, echoRunFunc())
	registerTaskTool(capturedAgent.Tools(), runner)

	input := json.RawMessage(`{"agent_name":"worker","prompt":"do stuff"}`)
	result, err := capturedAgent.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestWithDefinitions_TaskToolExecute_MissingAgentName(t *testing.T) {
	a := agent.NewAgent()
	runner := NewRunnerWithRunFunc(a, map[string]*Definition{
		"worker": {Name: "worker"},
	}, echoRunFunc())
	registerTaskTool(a.Tools(), runner)

	input := json.RawMessage(`{"agent_name":"","prompt":"do stuff"}`)
	result, err := a.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestWithDefinitions_TaskToolExecute_MissingPrompt(t *testing.T) {
	a := agent.NewAgent()
	runner := NewRunnerWithRunFunc(a, map[string]*Definition{
		"worker": {Name: "worker"},
	}, echoRunFunc())
	registerTaskTool(a.Tools(), runner)

	input := json.RawMessage(`{"agent_name":"worker","prompt":""}`)
	result, err := a.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestWithDefinitions_TaskToolExecute_DefinitionNotFound(t *testing.T) {
	a := agent.NewAgent()
	runner := NewRunnerWithRunFunc(a, map[string]*Definition{
		"worker": {Name: "worker"},
	}, echoRunFunc())
	registerTaskTool(a.Tools(), runner)

	input := json.RawMessage(`{"agent_name":"nonexistent","prompt":"do stuff"}`)
	result, err := a.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestWithDefinitions_TaskToolExecute_InvalidJSON(t *testing.T) {
	a := agent.NewAgent()
	runner := NewRunnerWithRunFunc(a, map[string]*Definition{
		"worker": {Name: "worker"},
	}, echoRunFunc())
	registerTaskTool(a.Tools(), runner)

	input := json.RawMessage(`{invalid json}`)
	result, err := a.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestWithDefinitions_TaskToolExecute_SubagentError(t *testing.T) {
	a := agent.NewAgent()
	runner := NewRunnerWithRunFunc(a, map[string]*Definition{
		"worker": {Name: "worker"},
	}, errorRunFunc("something broke"))
	registerTaskTool(a.Tools(), runner)

	input := json.RawMessage(`{"agent_name":"worker","prompt":"do stuff"}`)
	result, err := a.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestWithDefinitions_TaskToolExecute_EmptyOutput(t *testing.T) {
	a := agent.NewAgent()
	runner := NewRunnerWithRunFunc(a, map[string]*Definition{
		"worker": {Name: "worker"},
	}, successRunFunc(""))
	registerTaskTool(a.Tools(), runner)

	input := json.RawMessage(`{"agent_name":"worker","prompt":"do stuff"}`)
	result, err := a.Tools().Execute(context.Background(), "Task", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}
