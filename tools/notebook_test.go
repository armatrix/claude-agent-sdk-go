package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotebookEditTool_Name(t *testing.T) {
	tool := &NotebookEditTool{}
	assert.Equal(t, "NotebookEdit", tool.Name())
}

func TestNotebookEditTool_Description(t *testing.T) {
	tool := &NotebookEditTool{}
	assert.NotEmpty(t, tool.Description())
}

// sampleNotebook returns a minimal valid notebook JSON with two cells.
func sampleNotebook() notebookJSON {
	return notebookJSON{
		Cells: []notebookCell{
			{
				CellType: "code",
				Source:   []string{"print('hello')\n"},
				Metadata: map[string]interface{}{},
				ID:       "cell-0",
			},
			{
				CellType: "markdown",
				Source:   []string{"# Title\n"},
				Metadata: map[string]interface{}{},
				ID:       "cell-1",
			},
		},
		Metadata:      map[string]interface{}{},
		NBFormat:      4,
		NBFormatMinor: 5,
	}
}

func writeNotebook(t *testing.T, dir string, nb notebookJSON) string {
	t.Helper()
	path := filepath.Join(dir, "test.ipynb")
	data, err := json.MarshalIndent(nb, "", " ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
	return path
}

func readNotebook(t *testing.T, path string) notebookJSON {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var nb notebookJSON
	require.NoError(t, json.Unmarshal(data, &nb))
	return nb
}

func TestNotebookEditTool_Execute_ReplaceCell(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "print('updated')\n",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "replace mode")
	assert.Contains(t, extractText(result), "2 cells total")

	nb := readNotebook(t, path)
	assert.Equal(t, []string{"print('updated')\n"}, nb.Cells[0].Source)
}

func TestNotebookEditTool_Execute_InsertCell(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "x = 42\n",
		EditMode:     "insert",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "insert mode")
	assert.Contains(t, extractText(result), "3 cells total")

	nb := readNotebook(t, path)
	require.Len(t, nb.Cells, 3)
	// Inserted after cell 0
	assert.Equal(t, []string{"x = 42\n"}, nb.Cells[1].Source)
	assert.Equal(t, "code", nb.Cells[1].CellType) // default type
}

func TestNotebookEditTool_Execute_DeleteCell(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "", // not used for delete
		EditMode:     "delete",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "delete mode")
	assert.Contains(t, extractText(result), "1 cells total")

	nb := readNotebook(t, path)
	require.Len(t, nb.Cells, 1)
	assert.Equal(t, "cell-1", nb.Cells[0].ID)
}

func TestNotebookEditTool_Execute_CellByID(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellID:       "cell-1",
		NewSource:    "# Updated Title\n",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebook(t, path)
	assert.Equal(t, []string{"# Updated Title\n"}, nb.Cells[1].Source)
}

func TestNotebookEditTool_Execute_CellIDNotFound(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellID:       "nonexistent",
		NewSource:    "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "not found")
}

func TestNotebookEditTool_Execute_InvalidCellIndex(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 99
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "out of range")
}

func TestNotebookEditTool_Execute_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.ipynb")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "invalid notebook JSON")
}

func TestNotebookEditTool_Execute_EmptyPath(t *testing.T) {
	tool := &NotebookEditTool{}
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: "",
		NewSource:    "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "notebook_path is required")
}

func TestNotebookEditTool_Execute_NonexistentFile(t *testing.T) {
	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: "/nonexistent/notebook.ipynb",
		CellNumber:   &cellNum,
		NewSource:    "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "failed to read notebook")
}

func TestNotebookEditTool_Execute_DefaultEditModeIsReplace(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "replaced\n",
		// EditMode not set — should default to "replace"
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "replace mode")

	nb := readNotebook(t, path)
	assert.Equal(t, []string{"replaced\n"}, nb.Cells[0].Source)
}

func TestNotebookEditTool_Execute_InsertDefaultCellTypeIsCode(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "new_cell\n",
		EditMode:     "insert",
		// CellType not set — should default to "code"
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebook(t, path)
	assert.Equal(t, "code", nb.Cells[1].CellType)
}

func TestNotebookEditTool_Execute_InsertMarkdownCell(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 1
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "## Section\n",
		EditMode:     "insert",
		CellType:     "markdown",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebook(t, path)
	require.Len(t, nb.Cells, 3)
	assert.Equal(t, "markdown", nb.Cells[2].CellType)
}

func TestNotebookEditTool_Execute_UnknownEditMode(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "test",
		EditMode:     "bogus",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "unknown edit_mode")
}

func TestSplitSourceLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single line no newline",
			input:    "hello",
			expected: []string{"hello"},
		},
		{
			name:     "single line with newline",
			input:    "hello\n",
			expected: []string{"hello\n"},
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: []string{"line1\n", "line2\n", "line3"},
		},
		{
			name:     "trailing newline",
			input:    "a\nb\n",
			expected: []string{"a\n", "b\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitSourceLines(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNotebookEditTool_Execute_ReplaceCellType(t *testing.T) {
	dir := t.TempDir()
	path := writeNotebook(t, dir, sampleNotebook())

	tool := &NotebookEditTool{}
	cellNum := 0
	result, err := tool.Execute(context.Background(), NotebookEditInput{
		NotebookPath: path,
		CellNumber:   &cellNum,
		NewSource:    "# Now markdown\n",
		CellType:     "markdown",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	nb := readNotebook(t, path)
	assert.Equal(t, "markdown", nb.Cells[0].CellType)
}
