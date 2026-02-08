package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

func TestWithServers_RegistersBridgedTools(t *testing.T) {
	tools := []ToolInfo{
		{Name: "search", Description: "Search docs", InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "read", Description: "Read a doc", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	mock := newMockTransport(tools, nil)

	a := agent.NewAgent(
		WithTransports(map[string]Transport{
			"docs": mock,
		}),
	)

	names := a.Tools().Names()
	assert.Contains(t, names, "mcp__docs__search")
	assert.Contains(t, names, "mcp__docs__read")
}

func TestWithServers_Empty_NoTools(t *testing.T) {
	a := agent.NewAgent(
		WithServers(map[string]ServerConfig{}),
	)
	assert.Empty(t, a.Tools().Names())
}

func TestWithServers_Nil_NoTools(t *testing.T) {
	a := agent.NewAgent(
		WithServers(nil),
	)
	assert.Empty(t, a.Tools().Names())
}

func TestWithTransports_MultipleServers(t *testing.T) {
	tools1 := []ToolInfo{
		{Name: "tool_a", Description: "Tool A"},
	}
	tools2 := []ToolInfo{
		{Name: "tool_b", Description: "Tool B"},
		{Name: "tool_c", Description: "Tool C"},
	}

	a := agent.NewAgent(
		WithTransports(map[string]Transport{
			"srv1": newMockTransport(tools1, nil),
			"srv2": newMockTransport(tools2, nil),
		}),
	)

	names := a.Tools().Names()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "mcp__srv1__tool_a")
	assert.Contains(t, names, "mcp__srv2__tool_b")
	assert.Contains(t, names, "mcp__srv2__tool_c")
}

func TestWithTransports_CloseOnAgentClose(t *testing.T) {
	mock := newMockTransport(nil, nil)

	a := agent.NewAgent(
		WithTransports(map[string]Transport{
			"srv": mock,
		}),
	)

	// Transport should be connected after NewAgent.
	assert.True(t, mock.connected)

	// Agent.Close should disconnect the transport.
	err := a.Close()
	require.NoError(t, err)
	assert.False(t, mock.connected)
}

func TestWithTransports_Empty_NoCleanup(t *testing.T) {
	a := agent.NewAgent(
		WithTransports(map[string]Transport{}),
	)
	// Close should be no-op.
	assert.NoError(t, a.Close())
}

func TestWithTransports_ToolsExecutable(t *testing.T) {
	tools := []ToolInfo{
		{Name: "greet", Description: "Greet someone", InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)},
	}
	mock := newMockTransport(tools, nil)
	mock.callFn = func(_ context.Context, name string, args map[string]any) (string, error) {
		return "hello " + args["name"].(string), nil
	}

	a := agent.NewAgent(
		WithTransports(map[string]Transport{
			"greeter": mock,
		}),
	)

	input := json.RawMessage(`{"name":"world"}`)
	result, err := a.Tools().Execute(context.Background(), "mcp__greeter__greet", input)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}
