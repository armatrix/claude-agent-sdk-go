package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// EditInput defines the input for the Edit tool.
type EditInput struct {
	FilePath   string `json:"file_path" jsonschema:"required,description=The absolute path to the file to modify"`
	OldString  string `json:"old_string" jsonschema:"required,description=The text to replace"`
	NewString  string `json:"new_string" jsonschema:"required,description=The replacement text"`
	ReplaceAll bool   `json:"replace_all,omitempty" jsonschema:"description=Replace all occurrences"`
}

// EditTool performs exact string replacements in files.
type EditTool struct{}

var _ agent.Tool[EditInput] = (*EditTool)(nil)

func (t *EditTool) Name() string        { return "Edit" }
func (t *EditTool) Description() string  { return "Perform exact string replacements in files" }

func (t *EditTool) Execute(_ context.Context, input EditInput) (*agent.ToolResult, error) {
	if input.FilePath == "" {
		return agent.ErrorResult("file_path is required"), nil
	}
	if input.OldString == input.NewString {
		return agent.ErrorResult("old_string and new_string must be different"), nil
	}

	data, err := os.ReadFile(input.FilePath)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to read file: %s", err.Error())), nil
	}

	content := string(data)
	count := strings.Count(content, input.OldString)

	if count == 0 {
		return agent.ErrorResult("old_string not found in file"), nil
	}

	if !input.ReplaceAll && count > 1 {
		return agent.ErrorResult(fmt.Sprintf(
			"old_string appears %d times in file; use replace_all=true to replace all occurrences, or provide more context to make it unique",
			count,
		)), nil
	}

	var newContent string
	if input.ReplaceAll {
		newContent = strings.ReplaceAll(content, input.OldString, input.NewString)
	} else {
		newContent = strings.Replace(content, input.OldString, input.NewString, 1)
	}

	if err := os.WriteFile(input.FilePath, []byte(newContent), 0644); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to write file: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Successfully replaced %d occurrence(s) in %s", count, input.FilePath)), nil
}
