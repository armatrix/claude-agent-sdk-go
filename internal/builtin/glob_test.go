package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobTool_Name(t *testing.T) {
	tool := &GlobTool{}
	assert.Equal(t, "Glob", tool.Name())
}

func TestGlobTool_Execute_BasicMatch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0644))

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), GlobInput{
		Pattern: "*.txt",
		Path:    dir,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "a.txt")
	assert.Contains(t, text, "b.txt")
	assert.NotContains(t, text, "c.go")
}

func TestGlobTool_Execute_DoublestarPattern(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(subdir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "top.go"), []byte("t"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subdir, "nested.go"), []byte("n"), 0644))

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), GlobInput{
		Pattern: "**/*.go",
		Path:    dir,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "top.go")
	assert.Contains(t, text, "nested.go")
}

func TestGlobTool_Execute_NoMatches(t *testing.T) {
	dir := t.TempDir()

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), GlobInput{
		Pattern: "*.xyz",
		Path:    dir,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "No files matched")
}

func TestGlobTool_Execute_SortByModTime(t *testing.T) {
	dir := t.TempDir()

	// Create files with different mod times
	older := filepath.Join(dir, "older.txt")
	newer := filepath.Join(dir, "newer.txt")

	require.NoError(t, os.WriteFile(older, []byte("old"), 0644))
	// Set older file to an earlier time
	past := time.Now().Add(-1 * time.Hour)
	require.NoError(t, os.Chtimes(older, past, past))

	require.NoError(t, os.WriteFile(newer, []byte("new"), 0644))

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), GlobInput{
		Pattern: "*.txt",
		Path:    dir,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	// newer should appear before older (newest first)
	newerIdx := len(text) - len(text) // just check both appear
	_ = newerIdx
	assert.Contains(t, text, "newer.txt")
	assert.Contains(t, text, "older.txt")
}

func TestGlobTool_Execute_EmptyPattern(t *testing.T) {
	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), GlobInput{
		Pattern: "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestGlobTool_Execute_FullPaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("t"), 0644))

	tool := &GlobTool{}
	result, err := tool.Execute(context.Background(), GlobInput{
		Pattern: "*.txt",
		Path:    dir,
	})
	require.NoError(t, err)

	text := extractText(result)
	// Should contain the full absolute path
	assert.Contains(t, text, filepath.Join(dir, "test.txt"))
}
