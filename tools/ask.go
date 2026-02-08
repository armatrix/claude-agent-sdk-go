package tools

import (
	"context"
	"encoding/json"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// AskCallback is called when the LLM wants to ask the user a question.
type AskCallback func(ctx context.Context, question string, options []AskOption) (string, error)

// AskOption represents a selectable option for the user.
type AskOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// AskInput defines the input for the AskUserQuestion tool.
type AskInput struct {
	Question string          `json:"question" jsonschema:"required,description=The question to ask the user"`
	Options  json.RawMessage `json:"options,omitempty" jsonschema:"description=JSON array of option objects with label and description"`
}

// AskTool asks the user a question and returns their response.
type AskTool struct {
	Callback AskCallback
}

var _ agent.Tool[AskInput] = (*AskTool)(nil)

func (t *AskTool) Name() string        { return "AskUserQuestion" }
func (t *AskTool) Description() string  { return "Ask the user a question and wait for their response" }

func (t *AskTool) Execute(ctx context.Context, input AskInput) (*agent.ToolResult, error) {
	if input.Question == "" {
		return agent.ErrorResult("question is required"), nil
	}
	if t.Callback == nil {
		return agent.ErrorResult("ask callback not configured"), nil
	}

	var options []AskOption
	if len(input.Options) > 0 {
		if err := json.Unmarshal(input.Options, &options); err != nil {
			options = nil
		}
	}

	answer, err := t.Callback(ctx, input.Question, options)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("ask failed: %s", err.Error())), nil
	}
	return agent.TextResult(answer), nil
}
