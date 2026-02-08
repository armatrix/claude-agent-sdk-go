package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// NotebookEditInput defines the input for the NotebookEdit tool.
type NotebookEditInput struct {
	NotebookPath string `json:"notebook_path" jsonschema:"required,description=Absolute path to the .ipynb file"`
	NewSource    string `json:"new_source" jsonschema:"required,description=New source content for the cell"`
	CellNumber   *int   `json:"cell_number,omitempty" jsonschema:"description=0-indexed cell number to edit"`
	CellID       string `json:"cell_id,omitempty" jsonschema:"description=Cell ID to edit"`
	CellType     string `json:"cell_type,omitempty" jsonschema:"description=Cell type: code or markdown"`
	EditMode     string `json:"edit_mode,omitempty" jsonschema:"description=replace insert or delete"`
}

// NotebookEditTool manipulates Jupyter notebook cells.
type NotebookEditTool struct{}

var _ agent.Tool[NotebookEditInput] = (*NotebookEditTool)(nil)

func (t *NotebookEditTool) Name() string        { return "NotebookEdit" }
func (t *NotebookEditTool) Description() string  { return "Edit cells in a Jupyter notebook (.ipynb file)" }

// notebookJSON represents the top-level .ipynb structure.
type notebookJSON struct {
	Cells         []notebookCell         `json:"cells"`
	Metadata      map[string]interface{} `json:"metadata"`
	NBFormat      int                    `json:"nbformat"`
	NBFormatMinor int                    `json:"nbformat_minor"`
}

// notebookCell represents a single cell.
type notebookCell struct {
	CellType string                 `json:"cell_type"`
	Source   []string               `json:"source"`
	Metadata map[string]interface{} `json:"metadata"`
	ID       string                 `json:"id,omitempty"`
	Outputs  []interface{}          `json:"outputs,omitempty"`
}

func (t *NotebookEditTool) Execute(_ context.Context, input NotebookEditInput) (*agent.ToolResult, error) {
	if input.NotebookPath == "" {
		return agent.ErrorResult("notebook_path is required"), nil
	}

	// Read notebook
	data, err := os.ReadFile(input.NotebookPath)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to read notebook: %s", err.Error())), nil
	}

	var nb notebookJSON
	if err := json.Unmarshal(data, &nb); err != nil {
		return agent.ErrorResult(fmt.Sprintf("invalid notebook JSON: %s", err.Error())), nil
	}

	editMode := input.EditMode
	if editMode == "" {
		editMode = "replace"
	}

	// Find cell index
	cellIdx := -1
	if input.CellNumber != nil {
		cellIdx = *input.CellNumber
	} else if input.CellID != "" {
		for i, c := range nb.Cells {
			if c.ID == input.CellID {
				cellIdx = i
				break
			}
		}
		if cellIdx == -1 {
			return agent.ErrorResult(fmt.Sprintf("cell with ID %q not found", input.CellID)), nil
		}
	}

	// Convert source to lines
	sourceLines := splitSourceLines(input.NewSource)

	switch editMode {
	case "replace":
		if cellIdx < 0 || cellIdx >= len(nb.Cells) {
			return agent.ErrorResult(fmt.Sprintf("cell index %d out of range (0-%d)", cellIdx, len(nb.Cells)-1)), nil
		}
		nb.Cells[cellIdx].Source = sourceLines
		if input.CellType != "" {
			nb.Cells[cellIdx].CellType = input.CellType
		}

	case "insert":
		cellType := input.CellType
		if cellType == "" {
			cellType = "code"
		}
		newCell := notebookCell{
			CellType: cellType,
			Source:   sourceLines,
			Metadata: map[string]interface{}{},
		}
		if cellIdx < 0 {
			cellIdx = len(nb.Cells) // append at end
		}
		// Insert after cellIdx
		insertIdx := cellIdx + 1
		if insertIdx > len(nb.Cells) {
			insertIdx = len(nb.Cells)
		}
		nb.Cells = append(nb.Cells[:insertIdx], append([]notebookCell{newCell}, nb.Cells[insertIdx:]...)...)

	case "delete":
		if cellIdx < 0 || cellIdx >= len(nb.Cells) {
			return agent.ErrorResult(fmt.Sprintf("cell index %d out of range (0-%d)", cellIdx, len(nb.Cells)-1)), nil
		}
		nb.Cells = append(nb.Cells[:cellIdx], nb.Cells[cellIdx+1:]...)

	default:
		return agent.ErrorResult(fmt.Sprintf("unknown edit_mode: %s", editMode)), nil
	}

	// Write back
	output, err := json.MarshalIndent(nb, "", " ")
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to marshal notebook: %s", err.Error())), nil
	}

	if err := os.WriteFile(input.NotebookPath, output, 0644); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to write notebook: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Notebook edited successfully (%s mode, %d cells total)", editMode, len(nb.Cells))), nil
}

// splitSourceLines splits source into lines, preserving newlines at end of each line (Jupyter format).
func splitSourceLines(source string) []string {
	if source == "" {
		return []string{}
	}
	var lines []string
	current := ""
	for _, r := range source {
		current += string(r)
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
