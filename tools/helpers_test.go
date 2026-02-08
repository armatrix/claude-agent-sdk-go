package tools

import (
	"encoding/json"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// extractText gets the text content from a ToolResult's first content block.
func extractText(r *agent.ToolResult) string {
	if r == nil || len(r.Content) == 0 {
		return ""
	}
	// The content block is a union; marshal and extract the text field.
	b, err := json.Marshal(r.Content[0])
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	if text, ok := m["text"].(string); ok {
		return text
	}
	return ""
}
