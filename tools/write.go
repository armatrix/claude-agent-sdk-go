package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// WriteInput defines the input for the Write tool.
type WriteInput struct {
	FilePath string `json:"file_path" jsonschema:"required,description=The absolute path to the file to write"`
	Content  string `json:"content" jsonschema:"required,description=The content to write to the file"`
}

// WriteTool writes content to a file, creating parent directories if needed.
type WriteTool struct{}

var _ agent.Tool[WriteInput] = (*WriteTool)(nil)

func (t *WriteTool) Name() string        { return "Write" }
func (t *WriteTool) Description() string  { return "Write a file to the local filesystem" }

func (t *WriteTool) Execute(ctx context.Context, input WriteInput) (*agent.ToolResult, error) {
	if input.FilePath == "" {
		return agent.ErrorResult("file_path is required"), nil
	}

	resolved := resolvePath(ctx, input.FilePath)

	// Sandbox: check allowed directories
	if err := checkSandboxPath(ctx, resolved); err != nil {
		return agent.ErrorResult(err.Error()), nil
	}

	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to create directory: %s", err.Error())), nil
	}

	if err := os.WriteFile(resolved, []byte(input.Content), 0644); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to write file: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Successfully wrote to %s", resolved)), nil
}
