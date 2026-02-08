package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/internal/schema"
)

// SDKServer is an in-process MCP server that wraps Go functions as tools.
// Unlike external MCP servers, it runs in the same process — no subprocess,
// no JSON-RPC, no transport overhead.
//
// Usage:
//
//	srv := mcp.NewSDKServer("mytools")
//	mcp.AddTool(srv, "greet", "Greet someone", func(ctx context.Context, input GreetInput) (string, error) {
//	    return "Hello, " + input.Name, nil
//	})
//	a := agent.NewAgent(srv.AgentOption())
type SDKServer struct {
	name  string
	tools []sdkTool
}

type sdkTool struct {
	name        string
	description string
	schema      anthropic.ToolInputSchemaParam
	handler     func(ctx context.Context, input json.RawMessage) (string, error)
}

// NewSDKServer creates a new in-process MCP server with the given name.
// The name is used as the server component in bridged tool names (mcp__{name}__{tool}).
func NewSDKServer(name string) *SDKServer {
	return &SDKServer{name: name}
}

// Name returns the server name.
func (s *SDKServer) Name() string {
	return s.name
}

// ToolCount returns the number of registered tools.
func (s *SDKServer) ToolCount() int {
	return len(s.tools)
}

// ToolNames returns the original (un-namespaced) names of all registered tools.
func (s *SDKServer) ToolNames() []string {
	names := make([]string, len(s.tools))
	for i, t := range s.tools {
		names[i] = t.name
	}
	return names
}

// AddTool registers a typed Go function as an MCP tool.
// The input type T is used for automatic JSON Schema generation.
func AddTool[T any](s *SDKServer, name, description string, handler func(ctx context.Context, input T) (string, error)) {
	schemaParam := schema.Generate[T]()
	s.tools = append(s.tools, sdkTool{
		name:        name,
		description: description,
		schema:      schemaParam,
		handler: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var input T
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", fmt.Errorf("invalid input: %w", err)
			}
			return handler(ctx, input)
		},
	})
}

// AgentOption returns an AgentOption that registers all SDK server tools
// into the agent's ToolRegistry during initialization.
func (s *SDKServer) AgentOption() agent.AgentOption {
	return agent.WithOnInit(func(a *agent.Agent) {
		for _, t := range s.tools {
			toolName := BridgeToolName(s.name, t.name)
			handler := t.handler // capture for closure
			desc := t.description
			sch := t.schema
			a.Tools().RegisterRaw(
				toolName,
				desc,
				sch,
				func(ctx context.Context, raw json.RawMessage) (*agent.ToolResult, error) {
					result, err := handler(ctx, raw)
					if err != nil {
						return agent.ErrorResult(fmt.Sprintf("tool error: %s", err)), nil
					}
					return agent.TextResult(result), nil
				},
			)
		}
	})
}
