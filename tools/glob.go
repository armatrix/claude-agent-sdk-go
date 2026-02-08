package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// GlobInput defines the input for the Glob tool.
type GlobInput struct {
	Pattern string `json:"pattern" jsonschema:"required,description=The glob pattern to match files against"`
	Path    string `json:"path,omitempty" jsonschema:"description=The directory to search in"`
}

// GlobTool matches files using glob patterns.
type GlobTool struct{}

var _ agent.Tool[GlobInput] = (*GlobTool)(nil)

func (t *GlobTool) Name() string        { return "Glob" }
func (t *GlobTool) Description() string  { return "Fast file pattern matching tool" }

func (t *GlobTool) Execute(ctx context.Context, input GlobInput) (*agent.ToolResult, error) {
	if input.Pattern == "" {
		return agent.ErrorResult("pattern is required"), nil
	}

	basePath := input.Path
	if basePath == "" {
		if dir := agent.ContextWorkDir(ctx); dir != "" {
			basePath = dir
		} else {
			var err error
			basePath, err = os.Getwd()
			if err != nil {
				return agent.ErrorResult(fmt.Sprintf("failed to get working directory: %s", err.Error())), nil
			}
		}
	}

	// Resolve to absolute path
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("invalid path: %s", err.Error())), nil
	}

	fsys := os.DirFS(absBase)
	matches, err := doublestar.Glob(fsys, input.Pattern)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("glob error: %s", err.Error())), nil
	}

	if len(matches) == 0 {
		return agent.TextResult("No files matched the pattern."), nil
	}

	// Build full paths and get mod times for sorting
	type fileEntry struct {
		path    string
		modTime int64
	}
	entries := make([]fileEntry, 0, len(matches))
	for _, m := range matches {
		fullPath := filepath.Join(absBase, m)
		info, err := os.Stat(fullPath)
		if err != nil {
			// Skip files we can't stat
			continue
		}
		entries = append(entries, fileEntry{
			path:    fullPath,
			modTime: info.ModTime().UnixNano(),
		})
	}

	// Sort by modification time, newest first
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].modTime > entries[j].modTime
	})

	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.path)
		b.WriteByte('\n')
	}

	return agent.TextResult(b.String()), nil
}
