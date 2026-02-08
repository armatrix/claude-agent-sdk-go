package tools

import (
	"context"
	"testing"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubTool is a minimal tool for populating the registry in tests.
type stubToolInput struct {
	X string `json:"x"`
}

type stubTool struct {
	name string
	desc string
}

func (t *stubTool) Name() string        { return t.name }
func (t *stubTool) Description() string { return t.desc }

func (t *stubTool) Execute(_ context.Context, _ stubToolInput) (*agent.ToolResult, error) {
	return agent.TextResult("ok"), nil
}

func newSearchRegistry() *agent.ToolRegistry {
	r := agent.NewToolRegistry()
	agent.RegisterTool[stubToolInput](r, &stubTool{name: "ReadFile", desc: "Read a file from the filesystem"})
	agent.RegisterTool[stubToolInput](r, &stubTool{name: "WriteFile", desc: "Write content to a file"})
	agent.RegisterTool[stubToolInput](r, &stubTool{name: "BashExec", desc: "Execute a shell command"})
	agent.RegisterTool[stubToolInput](r, &stubTool{name: "WebSearch", desc: "Search the web for information"})
	return r
}

func TestToolSearchTool_FindsByName(t *testing.T) {
	registry := newSearchRegistry()
	tool := NewToolSearchTool(registry)

	result, err := tool.Execute(context.Background(), ToolSearchInput{Query: "bash"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "BashExec")
	assert.Contains(t, text, "Execute a shell command")
	assert.NotContains(t, text, "ReadFile")
}

func TestToolSearchTool_FindsByDescription(t *testing.T) {
	registry := newSearchRegistry()
	tool := NewToolSearchTool(registry)

	result, err := tool.Execute(context.Background(), ToolSearchInput{Query: "filesystem"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "ReadFile")
	assert.NotContains(t, text, "BashExec")
}

func TestToolSearchTool_NoMatch(t *testing.T) {
	registry := newSearchRegistry()
	tool := NewToolSearchTool(registry)

	result, err := tool.Execute(context.Background(), ToolSearchInput{Query: "deploy"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "No tools found matching: deploy")
}

func TestToolSearchTool_EmptyQuery(t *testing.T) {
	registry := newSearchRegistry()
	tool := NewToolSearchTool(registry)

	result, err := tool.Execute(context.Background(), ToolSearchInput{Query: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "query is required")
}

func TestToolSearchTool_CaseInsensitive(t *testing.T) {
	registry := newSearchRegistry()
	tool := NewToolSearchTool(registry)

	result, err := tool.Execute(context.Background(), ToolSearchInput{Query: "WEBSEARCH"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "WebSearch")
	assert.Contains(t, text, "Search the web for information")
}
