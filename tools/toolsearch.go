package tools

import (
	"context"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// ToolSearchInput defines the input for the ToolSearch meta-tool.
type ToolSearchInput struct {
	Query string `json:"query" jsonschema:"required,description=Search query to find tools by name or description"`
}

// ToolSearchTool searches the agent's tool registry for matching tools.
type ToolSearchTool struct {
	registry *agent.ToolRegistry
}

var _ agent.Tool[ToolSearchInput] = (*ToolSearchTool)(nil)

// NewToolSearchTool creates a ToolSearchTool bound to the given registry.
func NewToolSearchTool(registry *agent.ToolRegistry) *ToolSearchTool {
	return &ToolSearchTool{registry: registry}
}

func (t *ToolSearchTool) Name() string { return "ToolSearch" }
func (t *ToolSearchTool) Description() string {
	return "Search for available tools by name or description keyword"
}

func (t *ToolSearchTool) Execute(_ context.Context, input ToolSearchInput) (*agent.ToolResult, error) {
	if input.Query == "" {
		return agent.ErrorResult("query is required"), nil
	}

	query := strings.ToLower(input.Query)
	matches := t.registry.Search(query)

	if len(matches) == 0 {
		return agent.TextResult("No tools found matching: " + input.Query), nil
	}

	var sb strings.Builder
	sb.WriteString("Found tools:\n")
	for _, m := range matches {
		sb.WriteString("- ")
		sb.WriteString(m.Name)
		sb.WriteString(": ")
		sb.WriteString(m.Description)
		sb.WriteString("\n")
	}
	return agent.TextResult(sb.String()), nil
}
