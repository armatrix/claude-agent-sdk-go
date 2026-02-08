package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock Tool ---

type readInput struct {
	FilePath string `json:"file_path" jsonschema:"required,description=The absolute path to the file"`
	Offset   *int   `json:"offset,omitempty" jsonschema:"description=Line offset"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"description=Number of lines to read"`
}

type mockReadTool struct{}

func (t *mockReadTool) Name() string        { return "Read" }
func (t *mockReadTool) Description() string { return "Read a file from the filesystem" }

func (t *mockReadTool) Execute(_ context.Context, input readInput) (*ToolResult, error) {
	return TextResult("content of " + input.FilePath), nil
}

// --- Tests ---

func TestRegisterAndExecuteTool(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})

	input := json.RawMessage(`{"file_path": "/tmp/test.go"}`)
	result, err := registry.Execute(context.Background(), "Read", input)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)

	text := result.Content[0].GetText()
	require.NotNil(t, text)
	assert.Equal(t, "content of /tmp/test.go", *text)
}

func TestExecuteWithInvalidJSON(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})

	input := json.RawMessage(`{invalid json}`)
	result, err := registry.Execute(context.Background(), "Read", input)

	require.NoError(t, err, "invalid JSON should not return Go error, but tool error")
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestExecuteToolNotFound(t *testing.T) {
	registry := NewToolRegistry()

	_, err := registry.Execute(context.Background(), "NonExistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestListForAPI(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})

	tools := registry.ListForAPI()
	require.Len(t, tools, 1)

	tool := tools[0]
	require.NotNil(t, tool.OfTool)
	assert.Equal(t, "Read", tool.OfTool.Name)

	desc := tool.GetDescription()
	require.NotNil(t, desc)
	assert.Equal(t, "Read a file from the filesystem", *desc)

	schema := tool.GetInputSchema()
	require.NotNil(t, schema)
	assert.NotNil(t, schema.Properties)
	assert.Contains(t, schema.Required, "file_path")
}

func TestRegistryNames(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})

	names := registry.Names()
	assert.Equal(t, []string{"Read"}, names)
}

func TestRegistryGet(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})

	entry := registry.Get("Read")
	require.NotNil(t, entry)
	assert.Equal(t, "Read", entry.name)

	missing := registry.Get("Write")
	assert.Nil(t, missing)
}

// --- Second mock tool to test multiple registrations ---

type writeInput struct {
	FilePath string `json:"file_path" jsonschema:"required"`
	Content  string `json:"content" jsonschema:"required"`
}

type mockWriteTool struct{}

func (t *mockWriteTool) Name() string        { return "Write" }
func (t *mockWriteTool) Description() string { return "Write a file" }

func (t *mockWriteTool) Execute(_ context.Context, input writeInput) (*ToolResult, error) {
	return TextResult("wrote " + input.FilePath), nil
}

func TestMultipleToolRegistration(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})
	RegisterTool[writeInput](registry, &mockWriteTool{})

	names := registry.Names()
	assert.Equal(t, []string{"Read", "Write"}, names)

	tools := registry.ListForAPI()
	assert.Len(t, tools, 2)

	// Execute each
	r1, err := registry.Execute(context.Background(), "Read", json.RawMessage(`{"file_path":"/a"}`))
	require.NoError(t, err)
	assert.Equal(t, "content of /a", *r1.Content[0].GetText())

	r2, err := registry.Execute(context.Background(), "Write", json.RawMessage(`{"file_path":"/b","content":"x"}`))
	require.NoError(t, err)
	assert.Equal(t, "wrote /b", *r2.Content[0].GetText())
}

func TestTextResult(t *testing.T) {
	r := TextResult("hello")
	assert.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	assert.Equal(t, "hello", *r.Content[0].GetText())
}

func TestErrorResult(t *testing.T) {
	r := ErrorResult("something failed")
	assert.True(t, r.IsError)
	require.Len(t, r.Content, 1)
	assert.Equal(t, "something failed", *r.Content[0].GetText())
}

func TestListForAPISchemaSerializable(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool[readInput](registry, &mockReadTool{})

	tools := registry.ListForAPI()
	data, err := json.Marshal(tools)
	require.NoError(t, err)

	var parsed []map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	require.Len(t, parsed, 1)

	assert.Equal(t, "Read", parsed[0]["name"])
	assert.Equal(t, "Read a file from the filesystem", parsed[0]["description"])

	inputSchema, ok := parsed[0]["input_schema"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "object", inputSchema["type"])
}
