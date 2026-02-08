package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// GrepInput defines the input for the Grep tool.
type GrepInput struct {
	Pattern         string `json:"pattern" jsonschema:"required,description=The regex pattern to search for"`
	Path            string `json:"path,omitempty" jsonschema:"description=File or directory to search in"`
	OutputMode      string `json:"output_mode,omitempty" jsonschema:"description=Output mode: content or files_with_matches or count"`
	Glob            string `json:"glob,omitempty" jsonschema:"description=Glob pattern to filter files"`
	Type            string `json:"type,omitempty" jsonschema:"description=File type to search (e.g. go or py or js)"`
	Context         *int   `json:"context,omitempty" jsonschema:"description=Lines of context around matches"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty" jsonschema:"description=Case insensitive search"`
}

// GrepTool searches file contents using ripgrep.
type GrepTool struct{}

var _ agent.Tool[GrepInput] = (*GrepTool)(nil)

func (t *GrepTool) Name() string        { return "Grep" }
func (t *GrepTool) Description() string  { return "Search file contents using regex patterns" }

func (t *GrepTool) Execute(ctx context.Context, input GrepInput) (*agent.ToolResult, error) {
	if input.Pattern == "" {
		return agent.ErrorResult("pattern is required"), nil
	}

	// Check if rg is available
	rgPath, err := exec.LookPath("rg")
	if err != nil {
		return agent.ErrorResult("ripgrep (rg) is not installed. Install it with: brew install ripgrep (macOS) or apt install ripgrep (Linux)"), nil
	}

	args := buildRgArgs(input)

	cmd := exec.CommandContext(ctx, rgPath, args...)
	cmd.Dir = searchPath(input.Path)

	output, err := cmd.CombinedOutput()
	text := string(output)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// rg exit code 1 = no matches (not an error)
			if exitErr.ExitCode() == 1 {
				return agent.TextResult("No matches found."), nil
			}
			// rg exit code 2 = error
			return agent.ErrorResult(fmt.Sprintf("rg error: %s", text)), nil
		}
		return agent.ErrorResult(fmt.Sprintf("failed to run rg: %s", err.Error())), nil
	}

	if len(text) > maxOutputBytes {
		text = text[:maxOutputBytes] + "\n... [output truncated]"
	}

	return agent.TextResult(text), nil
}

func buildRgArgs(input GrepInput) []string {
	var args []string

	// Output mode
	switch input.OutputMode {
	case "content":
		// default rg behavior shows content
		args = append(args, "-n") // show line numbers
	case "count":
		args = append(args, "-c")
	case "files_with_matches", "":
		args = append(args, "-l")
	}

	if input.CaseInsensitive {
		args = append(args, "-i")
	}

	if input.Glob != "" {
		args = append(args, "--glob", input.Glob)
	}

	if input.Type != "" {
		args = append(args, "--type", input.Type)
	}

	if input.Context != nil && *input.Context > 0 {
		args = append(args, "-C", fmt.Sprintf("%d", *input.Context))
	}

	// Add pattern
	args = append(args, input.Pattern)

	// Add path if specified
	if input.Path != "" {
		args = append(args, input.Path)
	}

	return args
}

func searchPath(path string) string {
	if path != "" {
		return ""
	}
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
