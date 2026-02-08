package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteTool_Name(t *testing.T) {
	tool := &WriteTool{}
	assert.Equal(t, "Write", tool.Name())
}

func TestWriteTool_Execute_BasicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")

	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), WriteInput{
		FilePath: path,
		Content:  "hello world\n",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "Successfully wrote to")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", string(data))
}

func TestWriteTool_Execute_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.txt")
	require.NoError(t, os.WriteFile(path, []byte("old content"), 0644))

	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), WriteInput{
		FilePath: path,
		Content:  "new content",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

func TestWriteTool_Execute_CreateParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "output.txt")

	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), WriteInput{
		FilePath: path,
		Content:  "nested content",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "nested content", string(data))
}

func TestWriteTool_Execute_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")

	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), WriteInput{
		FilePath: path,
		Content:  "",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "", string(data))
}

func TestWriteTool_Execute_EmptyFilePath(t *testing.T) {
	tool := &WriteTool{}
	result, err := tool.Execute(context.Background(), WriteInput{
		FilePath: "",
		Content:  "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
