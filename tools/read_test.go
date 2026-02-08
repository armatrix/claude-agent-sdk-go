package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadTool_Name(t *testing.T) {
	tool := &ReadTool{}
	assert.Equal(t, "Read", tool.Name())
}

func TestReadTool_Execute_BasicRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line one\nline two\nline three\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{FilePath: path})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "1\tline one")
	assert.Contains(t, text, "2\tline two")
	assert.Contains(t, text, "3\tline three")
}

func TestReadTool_Execute_NonexistentFile(t *testing.T) {
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{FilePath: "/nonexistent/file.txt"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestReadTool_Execute_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	require.NoError(t, os.WriteFile(path, []byte(""), 0644))

	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{FilePath: path})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "(empty file)")
}

func TestReadTool_Execute_Offset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	offset := 3
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{
		FilePath: path,
		Offset:   &offset,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.NotContains(t, text, "line1")
	assert.NotContains(t, text, "line2")
	assert.Contains(t, text, "3\tline3")
	assert.Contains(t, text, "4\tline4")
	assert.Contains(t, text, "5\tline5")
}

func TestReadTool_Execute_Limit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	limit := 2
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{
		FilePath: path,
		Limit:    &limit,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "1\tline1")
	assert.Contains(t, text, "2\tline2")
	assert.NotContains(t, text, "line3")
}

func TestReadTool_Execute_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	offset := 2
	limit := 2
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{
		FilePath: path,
		Offset:   &offset,
		Limit:    &limit,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.NotContains(t, text, "line1")
	assert.Contains(t, text, "2\tline2")
	assert.Contains(t, text, "3\tline3")
	assert.NotContains(t, text, "line4")
}

func TestReadTool_Execute_LineTruncation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "long.txt")
	longLine := strings.Repeat("x", 3000)
	require.NoError(t, os.WriteFile(path, []byte(longLine+"\n"), 0644))

	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{FilePath: path})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, truncationSuffix)
	// The total content of the line (excluding line number prefix) should not exceed maxLineLength
	lines := strings.Split(strings.TrimSpace(text), "\n")
	require.Len(t, lines, 1)
}

func TestReadTool_Execute_EmptyFilePath(t *testing.T) {
	tool := &ReadTool{}
	result, err := tool.Execute(context.Background(), ReadInput{FilePath: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
