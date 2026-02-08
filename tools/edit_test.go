package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditTool_Name(t *testing.T) {
	tool := &EditTool{}
	assert.Equal(t, "Edit", tool.Name())
}

func TestEditTool_Execute_SingleReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0644))

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  path,
		OldString: "hello",
		NewString: "goodbye",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "1 occurrence(s)")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "goodbye world", string(data))
}

func TestEditTool_Execute_NonUniqueWithoutReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("foo bar foo baz foo"), 0644))

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  path,
		OldString: "foo",
		NewString: "qux",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "3 times")

	// File should be unchanged
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "foo bar foo baz foo", string(data))
}

func TestEditTool_Execute_ReplaceAll(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("foo bar foo baz foo"), 0644))

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:   path,
		OldString:  "foo",
		NewString:  "qux",
		ReplaceAll: true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "3 occurrence(s)")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "qux bar qux baz qux", string(data))
}

func TestEditTool_Execute_OldStringNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello world"), 0644))

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  path,
		OldString: "xyz",
		NewString: "abc",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "not found")
}

func TestEditTool_Execute_SameOldAndNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  path,
		OldString: "hello",
		NewString: "hello",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "must be different")
}

func TestEditTool_Execute_NonexistentFile(t *testing.T) {
	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  "/nonexistent/file.txt",
		OldString: "a",
		NewString: "b",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestEditTool_Execute_EmptyFilePath(t *testing.T) {
	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  "",
		OldString: "a",
		NewString: "b",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestEditTool_Execute_MultilineReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	content := "func main() {\n\tfmt.Println(\"hello\")\n}\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	tool := &EditTool{}
	result, err := tool.Execute(context.Background(), EditInput{
		FilePath:  path,
		OldString: "fmt.Println(\"hello\")",
		NewString: "fmt.Println(\"goodbye\")",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "func main() {\n\tfmt.Println(\"goodbye\")\n}\n", string(data))
}
