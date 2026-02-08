package builtin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBashTool_Name(t *testing.T) {
	tool := &BashTool{}
	assert.Equal(t, "Bash", tool.Name())
}

func TestBashTool_Execute_SimpleCommand(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "echo hello",
	})
	require.NoError(t, err)
	text := extractText(result)
	assert.Contains(t, text, "hello")
	assert.Equal(t, 0, result.Metadata["exit_code"])
}

func TestBashTool_Execute_ExitCode(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "exit 42",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Equal(t, 42, result.Metadata["exit_code"])
}

func TestBashTool_Execute_EmptyCommand(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "command is required")
}

func TestBashTool_Execute_Stderr(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "echo error_msg >&2",
	})
	require.NoError(t, err)
	text := extractText(result)
	assert.Contains(t, text, "error_msg")
}

func TestBashTool_Execute_Timeout(t *testing.T) {
	tool := &BashTool{}
	timeoutMs := 500
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "sleep 10",
		Timeout: &timeoutMs,
	})
	require.NoError(t, err)
	// Should either time out or have non-zero exit code
	assert.True(t, result.IsError)
}

func TestBashTool_Execute_MultilineOutput(t *testing.T) {
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "echo line1; echo line2; echo line3",
	})
	require.NoError(t, err)
	text := extractText(result)
	assert.Contains(t, text, "line1")
	assert.Contains(t, text, "line2")
	assert.Contains(t, text, "line3")
}

func TestBashTool_Execute_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	tool := &BashTool{}
	result, err := tool.Execute(context.Background(), BashInput{
		Command: "cd " + dir + " && pwd",
	})
	require.NoError(t, err)
	text := extractText(result)
	assert.Contains(t, text, dir)
}
