package agent

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testOutputStruct struct {
	Name  string `json:"name" jsonschema:"required,description=The name"`
	Score int    `json:"score" jsonschema:"required,description=A numeric score"`
}

func TestNewOutputFormat(t *testing.T) {
	schema := anthropic.ToolInputSchemaParam{
		Properties: map[string]any{
			"name": map[string]any{"type": "string"},
		},
		Required: []string{"name"},
	}
	format := NewOutputFormat("my_output", schema)
	assert.Equal(t, "my_output", format.Name)
	assert.Equal(t, []string{"name"}, format.Schema.Required)
}

func TestNewOutputFormatType(t *testing.T) {
	format := NewOutputFormatType[testOutputStruct]("test_output")
	assert.Equal(t, "test_output", format.Name)
	assert.Contains(t, format.Schema.Properties, "name")
	assert.Contains(t, format.Schema.Properties, "score")
}

func TestInjectOutputTool(t *testing.T) {
	format := NewOutputFormatType[testOutputStruct]("test_tool")

	params := anthropic.MessageNewParams{
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
	}

	injectOutputTool(&params, format)

	// Should have exactly one tool
	require.Len(t, params.Tools, 1)
	assert.Equal(t, "test_tool", params.Tools[0].OfTool.Name)

	// Should force tool_choice
	require.NotNil(t, params.ToolChoice.OfTool)
	assert.Equal(t, "test_tool", params.ToolChoice.OfTool.Name)
}

func TestInjectOutputTool_AppendsToExisting(t *testing.T) {
	format := NewOutputFormatType[testOutputStruct]("test_tool")

	params := anthropic.MessageNewParams{
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Tools: []anthropic.ToolUnionParam{
			{OfTool: &anthropic.ToolParam{Name: "existing_tool"}},
		},
	}

	injectOutputTool(&params, format)

	// Should have two tools now
	require.Len(t, params.Tools, 2)
	assert.Equal(t, "existing_tool", params.Tools[0].OfTool.Name)
	assert.Equal(t, "test_tool", params.Tools[1].OfTool.Name)
}

func TestExtractStructuredOutput(t *testing.T) {
	msg := anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type: "text",
				Text: "Here is the result",
			},
			{
				Type: "tool_use",
				ID:   "toolu_123",
				Name: "my_output",
				Input: json.RawMessage(`{"name":"Alice","score":95}`),
			},
		},
	}

	raw, err := ExtractStructuredOutput(msg, "my_output")
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Alice","score":95}`, string(raw))
}

func TestExtractStructuredOutput_NotFound(t *testing.T) {
	msg := anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{Type: "text", Text: "just text"},
		},
	}

	_, err := ExtractStructuredOutput(msg, "my_output")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractStructuredOutputTyped(t *testing.T) {
	msg := anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "toolu_456",
				Name:  "test_output",
				Input: json.RawMessage(`{"name":"Bob","score":42}`),
			},
		},
	}

	result, err := ExtractStructuredOutputTyped[testOutputStruct](msg, "test_output")
	require.NoError(t, err)
	assert.Equal(t, "Bob", result.Name)
	assert.Equal(t, 42, result.Score)
}

func TestExtractStructuredOutputTyped_InvalidJSON(t *testing.T) {
	msg := anthropic.Message{
		Content: []anthropic.ContentBlockUnion{
			{
				Type:  "tool_use",
				ID:    "toolu_789",
				Name:  "test_output",
				Input: json.RawMessage(`{"name":123}`),
			},
		},
	}

	// name field expects string but receives int — Go strict unmarshal fails
	_, err := ExtractStructuredOutputTyped[testOutputStruct](msg, "test_output")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal structured output")
}

func TestExtractStructuredOutputTyped_ToolNotFound(t *testing.T) {
	msg := anthropic.Message{
		Content: []anthropic.ContentBlockUnion{},
	}

	_, err := ExtractStructuredOutputTyped[testOutputStruct](msg, "missing")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWithOutputFormat_Option(t *testing.T) {
	format := NewOutputFormatType[testOutputStruct]("test")
	agent := NewAgent(WithOutputFormat(format))
	require.NotNil(t, agent.opts.outputFormat)
	assert.Equal(t, "test", agent.opts.outputFormat.Name)
}

func TestWithOutputFormatType_Option(t *testing.T) {
	agent := NewAgent(WithOutputFormatType[testOutputStruct]("typed_test"))
	require.NotNil(t, agent.opts.outputFormat)
	assert.Equal(t, "typed_test", agent.opts.outputFormat.Name)
}
