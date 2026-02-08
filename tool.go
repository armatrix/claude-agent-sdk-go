package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/armatrix/claude-agent-sdk-go/internal/schema"
)

// Tool is the generic interface for agent tools. The type parameter T defines
// the input struct that will be automatically deserialized from JSON.
type Tool[T any] interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input T) (*ToolResult, error)
}

// ToolResult is the output of a tool execution.
type ToolResult struct {
	Content  []anthropic.ContentBlockParamUnion
	IsError  bool
	Metadata map[string]any
}

// TextResult is a convenience constructor for a text-only tool result.
func TextResult(text string) *ToolResult {
	return &ToolResult{
		Content: []anthropic.ContentBlockParamUnion{
			anthropic.NewTextBlock(text),
		},
	}
}

// ErrorResult is a convenience constructor for an error tool result.
func ErrorResult(text string) *ToolResult {
	return &ToolResult{
		Content: []anthropic.ContentBlockParamUnion{
			anthropic.NewTextBlock(text),
		},
		IsError: true,
	}
}

// toolEntry is the type-erased wrapper stored in the registry.
type toolEntry struct {
	name        string
	description string
	schema      anthropic.ToolInputSchemaParam
	execute     func(ctx context.Context, raw json.RawMessage) (*ToolResult, error)
}

// ToolRegistry manages registered tools. It is concurrent-safe.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*toolEntry
	order []string // preserve registration order
}

// NewToolRegistry creates a new empty ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*toolEntry),
	}
}

// RegisterTool registers a generic tool into the registry.
// The input type T is used to auto-generate a JSON Schema.
func RegisterTool[T any](r *ToolRegistry, tool Tool[T]) {
	s := schema.Generate[T]()
	entry := &toolEntry{
		name:        tool.Name(),
		description: tool.Description(),
		schema:      s,
		execute: func(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
			var input T
			if err := json.Unmarshal(raw, &input); err != nil {
				return ErrorResult(fmt.Sprintf("invalid input: %s", err.Error())), nil
			}
			return tool.Execute(ctx, input)
		},
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[entry.name]; !exists {
		r.order = append(r.order, entry.name)
	}
	r.tools[entry.name] = entry
}

// RegisterRaw registers a tool with a pre-built schema and execute function.
// This is used by MCP bridged tools and other dynamic tool sources that
// don't use the generic Tool[T] interface.
func (r *ToolRegistry) RegisterRaw(
	name, description string,
	inputSchema anthropic.ToolInputSchemaParam,
	execute func(ctx context.Context, raw json.RawMessage) (*ToolResult, error),
) {
	entry := &toolEntry{
		name:        name,
		description: description,
		schema:      inputSchema,
		execute:     execute,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = entry
}

// Execute runs a tool by name with the given raw JSON input.
func (r *ToolRegistry) Execute(ctx context.Context, name string, input json.RawMessage) (*ToolResult, error) {
	r.mu.RLock()
	entry, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return entry.execute(ctx, input)
}

// ListForAPI returns the registered tools in the format expected by the Anthropic API.
func (r *ToolRegistry) ListForAPI() []anthropic.ToolUnionParam {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]anthropic.ToolUnionParam, 0, len(r.tools))
	for _, name := range r.order {
		entry := r.tools[name]
		result = append(result, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        entry.name,
				Description: param.NewOpt(entry.description),
				InputSchema: entry.schema,
			},
		})
	}
	return result
}

// Get returns a tool entry by name, or nil if not found.
func (r *ToolRegistry) Get(name string) *toolEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// Names returns the names of all registered tools in registration order.
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, len(r.order))
	copy(names, r.order)
	return names
}

// ToolSearchMatch represents a tool found by search.
type ToolSearchMatch struct {
	Name        string
	Description string
}

// Search finds tools whose name or description contains the query (case-insensitive).
func (r *ToolRegistry) Search(query string) []ToolSearchMatch {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := strings.ToLower(query)
	var matches []ToolSearchMatch
	for _, name := range r.order {
		entry := r.tools[name]
		if strings.Contains(strings.ToLower(entry.name), q) ||
			strings.Contains(strings.ToLower(entry.description), q) {
			matches = append(matches, ToolSearchMatch{
				Name:        entry.name,
				Description: entry.description,
			})
		}
	}
	return matches
}
