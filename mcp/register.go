package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// RegisterBridgedTools discovers tools from all connected MCP servers and
// registers them into the agent's ToolRegistry. Each tool is namespaced
// as mcp__{server}__{tool} to avoid collisions.
//
// This is the primary integration point between MCP and the Agent:
//
//	mgr := mcp.NewManager(configs)
//	mgr.Connect(ctx)
//	mcp.RegisterBridgedTools(registry, mgr)
func RegisterBridgedTools(registry *agent.ToolRegistry, mgr *Manager) {
	for _, bt := range mgr.BridgedTools() {
		schema := buildSchema(bt.InputSchema)
		fullName := bt.FullName

		registry.RegisterRaw(
			bt.FullName,
			bt.Description,
			schema,
			func(ctx context.Context, raw json.RawMessage) (*agent.ToolResult, error) {
				result, err := mgr.CallToolRaw(ctx, fullName, raw)
				if err != nil {
					return agent.ErrorResult(fmt.Sprintf("MCP tool error: %s", err.Error())), nil
				}
				return agent.TextResult(result), nil
			},
		)
	}
}

// buildSchema constructs a ToolInputSchemaParam from raw JSON schema bytes.
func buildSchema(raw json.RawMessage) anthropic.ToolInputSchemaParam {
	schema := anthropic.ToolInputSchemaParam{}

	if len(raw) == 0 {
		return schema
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return schema
	}

	if props, ok := parsed["properties"]; ok {
		schema.Properties = props
	}
	if req, ok := parsed["required"].([]any); ok {
		required := make([]string, 0, len(req))
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
		schema.Required = required
	}

	return schema
}
