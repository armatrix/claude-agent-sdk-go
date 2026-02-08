package builtin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireRg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep (rg) not installed, skipping grep tests")
	}
}

func TestGrepTool_Name(t *testing.T) {
	tool := &GrepTool{}
	assert.Equal(t, "Grep", tool.Name())
}

func TestGrepTool_Execute_FilesWithMatches(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("goodbye world\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.txt"), []byte("no match here\n"), 0644))

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern: "world",
		Path:    dir,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "a.txt")
	assert.Contains(t, text, "b.txt")
	assert.NotContains(t, text, "c.txt")
}

func TestGrepTool_Execute_ContentMode(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("line1\nhello world\nline3\n"), 0644))

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern:    "hello",
		Path:       dir,
		OutputMode: "content",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "hello world")
}

func TestGrepTool_Execute_CountMode(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("foo\nfoo\nbar\nfoo\n"), 0644))

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern:    "foo",
		Path:       dir,
		OutputMode: "count",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "3")
}

func TestGrepTool_Execute_NoMatches(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello world\n"), 0644))

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern: "zzzznotfound",
		Path:    dir,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "No matches found")
}

func TestGrepTool_Execute_CaseInsensitive(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("Hello World\n"), 0644))

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern:         "hello",
		Path:            dir,
		CaseInsensitive: true,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "test.txt")
}

func TestGrepTool_Execute_GlobFilter(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("hello\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello\n"), 0644))

	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern: "hello",
		Path:    dir,
		Glob:    "*.go",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "a.go")
	assert.NotContains(t, text, "b.txt")
}

func TestGrepTool_Execute_EmptyPattern(t *testing.T) {
	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern: "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "pattern is required")
}

func TestGrepTool_Execute_Context(t *testing.T) {
	requireRg(t)

	dir := t.TempDir()
	content := "line1\nline2\nmatch_here\nline4\nline5\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644))

	ctx := 1
	tool := &GrepTool{}
	result, err := tool.Execute(context.Background(), GrepInput{
		Pattern:    "match_here",
		Path:       dir,
		OutputMode: "content",
		Context:    &ctx,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "line2")
	assert.Contains(t, text, "match_here")
	assert.Contains(t, text, "line4")
}
