package mcp

// Bridge converts MCP tools into agent-compatible tool entries.
// Tool naming convention: mcp__{server}__{tool} (aligned with Claude Code).

// BridgedTool represents an MCP tool adapted for use in the agent's ToolRegistry.
type BridgedTool struct {
	// ServerName is the MCP server this tool belongs to.
	ServerName string

	// ToolName is the original tool name from the MCP server.
	ToolName string

	// FullName is the namespaced name: mcp__{server}__{tool}.
	FullName string

	// Description is the tool's description from the MCP server.
	Description string
}

// BridgeToolName returns the namespaced tool name for an MCP tool.
func BridgeToolName(serverName, toolName string) string {
	return "mcp__" + serverName + "__" + toolName
}
