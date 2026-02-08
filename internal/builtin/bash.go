package builtin

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/creack/pty"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

const (
	defaultBashTimeoutMs = 120_000
	maxBashTimeoutMs     = 600_000
	maxOutputBytes       = 30_000
)

// BashInput defines the input for the Bash tool.
type BashInput struct {
	Command         string `json:"command" jsonschema:"required,description=The command to execute"`
	Description     string `json:"description,omitempty" jsonschema:"description=Description of what this command does"`
	Timeout         *int   `json:"timeout,omitempty" jsonschema:"description=Timeout in milliseconds (max 600000)"`
	RunInBackground bool   `json:"run_in_background,omitempty" jsonschema:"description=Run command in background"`
}

// BashTool executes shell commands.
type BashTool struct{}

var _ agent.Tool[BashInput] = (*BashTool)(nil)

func (t *BashTool) Name() string        { return "Bash" }
func (t *BashTool) Description() string  { return "Execute a bash command" }

func (t *BashTool) Execute(ctx context.Context, input BashInput) (*agent.ToolResult, error) {
	if input.Command == "" {
		return agent.ErrorResult("command is required"), nil
	}

	timeoutMs := defaultBashTimeoutMs
	if input.Timeout != nil {
		timeoutMs = *input.Timeout
		if timeoutMs <= 0 {
			timeoutMs = defaultBashTimeoutMs
		}
		if timeoutMs > maxBashTimeoutMs {
			timeoutMs = maxBashTimeoutMs
		}
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", input.Command)

	// Use PTY for more realistic output capture
	ptmx, err := pty.Start(cmd)
	if err != nil {
		// Fallback to regular execution if PTY fails
		return t.executeWithoutPTY(cmdCtx, input.Command)
	}
	defer ptmx.Close()

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, ptmx) // PTY read returns EIO on process exit, ignore

	// Wait for the command to finish
	waitErr := cmd.Wait()

	output := buf.String()
	if len(output) > maxOutputBytes {
		output = output[:maxOutputBytes] + "\n... [output truncated]"
	}

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if cmdCtx.Err() == context.DeadlineExceeded {
			return agent.ErrorResult(fmt.Sprintf("command timed out after %dms", timeoutMs)), nil
		} else {
			exitCode = -1
		}
	}

	result := agent.TextResult(output)
	result.Metadata = map[string]any{
		"exit_code": exitCode,
	}
	if exitCode != 0 {
		result.IsError = true
	}

	return result, nil
}

func (t *BashTool) executeWithoutPTY(ctx context.Context, command string) (*agent.ToolResult, error) {
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	output, err := cmd.CombinedOutput()

	text := string(output)
	if len(text) > maxOutputBytes {
		text = text[:maxOutputBytes] + "\n... [output truncated]"
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return agent.ErrorResult("command timed out"), nil
		} else {
			exitCode = -1
		}
	}

	result := agent.TextResult(text)
	result.Metadata = map[string]any{
		"exit_code": exitCode,
	}
	if exitCode != 0 {
		result.IsError = true
	}

	return result, nil
}
