package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

const (
	defaultReadLimit     = 2000
	maxLineLength        = 2000
	truncationSuffix     = "... [truncated]"
	lineNumberTabWidth   = 6 // right-justified width for line numbers
)

// ReadInput defines the input for the Read tool.
type ReadInput struct {
	FilePath string `json:"file_path" jsonschema:"required,description=The absolute path to the file to read"`
	Offset   *int   `json:"offset,omitempty" jsonschema:"description=The line number to start reading from (1-based)"`
	Limit    *int   `json:"limit,omitempty" jsonschema:"description=The number of lines to read"`
}

// ReadTool reads file content with optional offset and limit.
type ReadTool struct{}

var _ agent.Tool[ReadInput] = (*ReadTool)(nil)

func (t *ReadTool) Name() string        { return "Read" }
func (t *ReadTool) Description() string  { return "Read a file from the local filesystem" }

func (t *ReadTool) Execute(_ context.Context, input ReadInput) (*agent.ToolResult, error) {
	if input.FilePath == "" {
		return agent.ErrorResult("file_path is required"), nil
	}

	f, err := os.Open(input.FilePath)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to open file: %s", err.Error())), nil
	}
	defer f.Close()

	limit := defaultReadLimit
	if input.Limit != nil && *input.Limit > 0 {
		limit = *input.Limit
	}

	offset := 1
	if input.Offset != nil && *input.Offset > 0 {
		offset = *input.Offset
	}

	scanner := bufio.NewScanner(f)
	// Increase scanner buffer to handle long lines
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var b strings.Builder
	lineNum := 0
	linesOutput := 0

	for scanner.Scan() {
		lineNum++
		if lineNum < offset {
			continue
		}
		if linesOutput >= limit {
			break
		}

		line := scanner.Text()
		if len(line) > maxLineLength {
			line = line[:maxLineLength-len(truncationSuffix)] + truncationSuffix
		}

		fmt.Fprintf(&b, "%*d\t%s\n", lineNumberTabWidth, lineNum, line)
		linesOutput++
	}

	if err := scanner.Err(); err != nil {
		return agent.ErrorResult(fmt.Sprintf("error reading file: %s", err.Error())), nil
	}

	if b.Len() == 0 {
		return agent.TextResult("(empty file)"), nil
	}

	return agent.TextResult(b.String()), nil
}
