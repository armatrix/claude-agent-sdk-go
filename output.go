package agent

import (
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/armatrix/claude-agent-sdk-go/internal/schema"
)

// OutputFormat defines a structured output format using the hidden tool pattern.
// The Anthropic API achieves structured output by injecting a hidden tool with
// the desired JSON schema and forcing tool_choice to that tool.
type OutputFormat struct {
	Name   string                        // Tool name (e.g., "structured_output")
	Schema anthropic.ToolInputSchemaParam // JSON Schema for the output
}

// NewOutputFormat creates an OutputFormat with the given name and schema.
func NewOutputFormat(name string, schema anthropic.ToolInputSchemaParam) OutputFormat {
	return OutputFormat{Name: name, Schema: schema}
}

// NewOutputFormatType creates an OutputFormat from a Go struct type T.
// The schema is auto-generated from struct tags.
func NewOutputFormatType[T any](name string) OutputFormat {
	return OutputFormat{
		Name:   name,
		Schema: schema.Generate[T](),
	}
}

// injectOutputTool adds the hidden structured_output tool to the tool list
// and sets tool_choice to force its use.
func injectOutputTool(params *anthropic.MessageNewParams, format OutputFormat) {
	hiddenTool := anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        format.Name,
			Description: param.NewOpt("Return structured output matching the schema"),
			InputSchema: format.Schema,
		},
	}
	params.Tools = append(params.Tools, hiddenTool)
	params.ToolChoice = anthropic.ToolChoiceParamOfTool(format.Name)
}

// ExtractStructuredOutput extracts the structured output from a tool_use block
// in the assistant message. Returns the raw JSON of the tool input.
func ExtractStructuredOutput(msg anthropic.Message, toolName string) (json.RawMessage, error) {
	for _, block := range msg.Content {
		if block.Type != "tool_use" {
			continue
		}
		if block.Name == toolName {
			return json.RawMessage(block.Input), nil
		}
	}
	return nil, fmt.Errorf("structured output tool %q not found in response", toolName)
}

// ExtractStructuredOutputTyped extracts and unmarshals structured output into type T.
func ExtractStructuredOutputTyped[T any](msg anthropic.Message, toolName string) (*T, error) {
	raw, err := ExtractStructuredOutput(msg, toolName)
	if err != nil {
		return nil, err
	}
	var result T
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("unmarshal structured output: %w", err)
	}
	return &result, nil
}
