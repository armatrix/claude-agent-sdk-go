package tools

import (
	"context"
	"fmt"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/mcp"
)

// ListMcpResourcesInput defines the input for the ListMcpResources tool.
type ListMcpResourcesInput struct {
	ServerName string `json:"server_name" jsonschema:"required,description=MCP server name to list resources from"`
}

// ListMcpResourcesTool lists resources available on an MCP server.
type ListMcpResourcesTool struct {
	manager *mcp.Manager
}

var _ agent.Tool[ListMcpResourcesInput] = (*ListMcpResourcesTool)(nil)

// NewListMcpResourcesTool creates a ListMcpResourcesTool backed by the given Manager.
func NewListMcpResourcesTool(m *mcp.Manager) *ListMcpResourcesTool {
	return &ListMcpResourcesTool{manager: m}
}

func (t *ListMcpResourcesTool) Name() string { return "ListMcpResources" }

func (t *ListMcpResourcesTool) Description() string {
	return "List resources available on an MCP server"
}

func (t *ListMcpResourcesTool) Execute(ctx context.Context, input ListMcpResourcesInput) (*agent.ToolResult, error) {
	if input.ServerName == "" {
		return agent.ErrorResult("server_name is required"), nil
	}

	resources, err := t.manager.ListResources(ctx, input.ServerName)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to list resources: %s", err.Error())), nil
	}

	if len(resources) == 0 {
		return agent.TextResult("No resources available."), nil
	}

	var b strings.Builder
	for i, r := range resources {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- %s (%s)", r.Name, r.URI)
		if r.Description != "" {
			fmt.Fprintf(&b, ": %s", r.Description)
		}
		if r.MIMEType != "" {
			fmt.Fprintf(&b, " [%s]", r.MIMEType)
		}
	}

	return agent.TextResult(b.String()), nil
}

// ReadMcpResourceInput defines the input for the ReadMcpResource tool.
type ReadMcpResourceInput struct {
	ServerName string `json:"server_name" jsonschema:"required,description=MCP server name to read resource from"`
	URI        string `json:"uri" jsonschema:"required,description=Resource URI to read"`
}

// ReadMcpResourceTool reads a specific resource from an MCP server.
type ReadMcpResourceTool struct {
	manager *mcp.Manager
}

var _ agent.Tool[ReadMcpResourceInput] = (*ReadMcpResourceTool)(nil)

// NewReadMcpResourceTool creates a ReadMcpResourceTool backed by the given Manager.
func NewReadMcpResourceTool(m *mcp.Manager) *ReadMcpResourceTool {
	return &ReadMcpResourceTool{manager: m}
}

func (t *ReadMcpResourceTool) Name() string { return "ReadMcpResource" }

func (t *ReadMcpResourceTool) Description() string {
	return "Read a resource from an MCP server by URI"
}

func (t *ReadMcpResourceTool) Execute(ctx context.Context, input ReadMcpResourceInput) (*agent.ToolResult, error) {
	if input.ServerName == "" {
		return agent.ErrorResult("server_name is required"), nil
	}
	if input.URI == "" {
		return agent.ErrorResult("uri is required"), nil
	}

	content, err := t.manager.ReadResource(ctx, input.ServerName, input.URI)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to read resource: %s", err.Error())), nil
	}

	return agent.TextResult(content), nil
}
