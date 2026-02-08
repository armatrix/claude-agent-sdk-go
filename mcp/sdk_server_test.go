package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

func TestNewSDKServer(t *testing.T) {
	srv := NewSDKServer("mytools")

	assert.Equal(t, "mytools", srv.Name())
	assert.Equal(t, 0, srv.ToolCount())
	assert.Empty(t, srv.ToolNames())
}

func TestAddTool_RegistersTool(t *testing.T) {
	type GreetInput struct {
		Name string `json:"name" jsonschema:"description=Name to greet,required"`
	}

	srv := NewSDKServer("greeter")
	AddTool(srv, "greet", "Greet someone", func(ctx context.Context, input GreetInput) (string, error) {
		return "Hello, " + input.Name, nil
	})

	assert.Equal(t, 1, srv.ToolCount())
	assert.Equal(t, []string{"greet"}, srv.ToolNames())
}

func TestSDKServer_AgentOption_RegistersTools(t *testing.T) {
	type Input struct {
		Query string `json:"query"`
	}

	srv := NewSDKServer("search")
	AddTool(srv, "find", "Find items", func(ctx context.Context, input Input) (string, error) {
		return "found", nil
	})

	a := agent.NewAgent(srv.AgentOption())

	names := a.Tools().Names()
	assert.Contains(t, names, "mcp__search__find")
}

func TestSDKServer_ToolExecution(t *testing.T) {
	type EchoInput struct {
		Message string `json:"message"`
	}

	srv := NewSDKServer("echo-server")
	AddTool(srv, "echo", "Echo a message", func(ctx context.Context, input EchoInput) (string, error) {
		return "echo: " + input.Message, nil
	})

	a := agent.NewAgent(srv.AgentOption())

	input := json.RawMessage(`{"message":"hello"}`)
	result, err := a.Tools().Execute(context.Background(), "mcp__echo-server__echo", input)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Extract text from content blocks
	require.Len(t, result.Content, 1)
	assert.Equal(t, "echo: hello", result.Content[0].OfText.Text)
}

func TestSDKServer_ToolExecution_Error(t *testing.T) {
	type Input struct {
		X int `json:"x"`
	}

	srv := NewSDKServer("math")
	AddTool(srv, "divide", "Divide by zero", func(ctx context.Context, input Input) (string, error) {
		if input.X == 0 {
			return "", fmt.Errorf("division by zero")
		}
		return "ok", nil
	})

	a := agent.NewAgent(srv.AgentOption())

	input := json.RawMessage(`{"x":0}`)
	result, err := a.Tools().Execute(context.Background(), "mcp__math__divide", input)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Equal(t, "tool error: division by zero", result.Content[0].OfText.Text)
}

func TestSDKServer_ToolExecution_InvalidJSON(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}

	srv := NewSDKServer("srv")
	AddTool(srv, "tool", "A tool", func(ctx context.Context, input Input) (string, error) {
		return input.Name, nil
	})

	a := agent.NewAgent(srv.AgentOption())

	input := json.RawMessage(`{invalid json}`)
	result, err := a.Tools().Execute(context.Background(), "mcp__srv__tool", input)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestSDKServer_MultipleTools(t *testing.T) {
	type AddInput struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	type MulInput struct {
		A int `json:"a"`
		B int `json:"b"`
	}

	srv := NewSDKServer("calc")
	AddTool(srv, "add", "Add numbers", func(ctx context.Context, input AddInput) (string, error) {
		return fmt.Sprintf("%d", input.A+input.B), nil
	})
	AddTool(srv, "mul", "Multiply numbers", func(ctx context.Context, input MulInput) (string, error) {
		return fmt.Sprintf("%d", input.A*input.B), nil
	})

	assert.Equal(t, 2, srv.ToolCount())
	assert.Equal(t, []string{"add", "mul"}, srv.ToolNames())

	a := agent.NewAgent(srv.AgentOption())

	names := a.Tools().Names()
	assert.Contains(t, names, "mcp__calc__add")
	assert.Contains(t, names, "mcp__calc__mul")

	// Execute add
	result, err := a.Tools().Execute(context.Background(), "mcp__calc__add", json.RawMessage(`{"a":2,"b":3}`))
	require.NoError(t, err)
	assert.Equal(t, "5", result.Content[0].OfText.Text)

	// Execute mul
	result, err = a.Tools().Execute(context.Background(), "mcp__calc__mul", json.RawMessage(`{"a":4,"b":5}`))
	require.NoError(t, err)
	assert.Equal(t, "20", result.Content[0].OfText.Text)
}

func TestSDKServer_ToolNaming(t *testing.T) {
	type Input struct{}

	tests := []struct {
		server string
		tool   string
		want   string
	}{
		{"myserver", "mytool", "mcp__myserver__mytool"},
		{"my-server", "my-tool", "mcp__my-server__my-tool"},
		{"srv", "tool_name", "mcp__srv__tool_name"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			srv := NewSDKServer(tt.server)
			AddTool(srv, tt.tool, "desc", func(ctx context.Context, input Input) (string, error) {
				return "ok", nil
			})

			a := agent.NewAgent(srv.AgentOption())
			names := a.Tools().Names()
			assert.Contains(t, names, tt.want)
		})
	}
}

func TestAddTool_SchemaGeneration(t *testing.T) {
	type SearchInput struct {
		Query  string `json:"query" jsonschema:"description=Search query,required"`
		Limit  int    `json:"limit" jsonschema:"description=Max results"`
	}

	srv := NewSDKServer("search")
	AddTool(srv, "search", "Search", func(ctx context.Context, input SearchInput) (string, error) {
		return "ok", nil
	})

	require.Equal(t, 1, srv.ToolCount())

	// Verify schema was generated (not nil/empty)
	tool := srv.tools[0]
	assert.NotNil(t, tool.schema.Properties)

	// Check that properties map contains expected fields
	props, ok := tool.schema.Properties.(map[string]any)
	require.True(t, ok, "properties should be map[string]any")
	assert.Contains(t, props, "query")
	assert.Contains(t, props, "limit")
}

func TestSDKServer_MultipleServers(t *testing.T) {
	type Input struct {
		X string `json:"x"`
	}

	srv1 := NewSDKServer("alpha")
	AddTool(srv1, "tool1", "Tool 1", func(ctx context.Context, input Input) (string, error) {
		return "alpha:" + input.X, nil
	})

	srv2 := NewSDKServer("beta")
	AddTool(srv2, "tool1", "Tool 1 on beta", func(ctx context.Context, input Input) (string, error) {
		return "beta:" + input.X, nil
	})

	a := agent.NewAgent(srv1.AgentOption(), srv2.AgentOption())

	names := a.Tools().Names()
	assert.Contains(t, names, "mcp__alpha__tool1")
	assert.Contains(t, names, "mcp__beta__tool1")

	// Execute on alpha
	result, err := a.Tools().Execute(context.Background(), "mcp__alpha__tool1", json.RawMessage(`{"x":"test"}`))
	require.NoError(t, err)
	assert.Equal(t, "alpha:test", result.Content[0].OfText.Text)

	// Execute on beta
	result, err = a.Tools().Execute(context.Background(), "mcp__beta__tool1", json.RawMessage(`{"x":"test"}`))
	require.NoError(t, err)
	assert.Equal(t, "beta:test", result.Content[0].OfText.Text)
}
