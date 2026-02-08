package tools

import (
	"context"
	"fmt"
	"sync"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// TodoInput defines the input for the TodoWrite tool.
type TodoInput struct {
	Todos []TodoItem `json:"todos" jsonschema:"required,description=The complete todo list to write"`
}

// TodoItem represents a single todo entry.
type TodoItem struct {
	ID      string `json:"id" jsonschema:"required,description=Unique identifier"`
	Content string `json:"content" jsonschema:"required,description=Task description"`
	Status  string `json:"status" jsonschema:"required,description=pending|in_progress|completed"`
}

// TodoTool manages a concurrent-safe in-memory todo list.
type TodoTool struct {
	mu    sync.RWMutex
	todos []TodoItem
}

var _ agent.Tool[TodoInput] = (*TodoTool)(nil)

func (t *TodoTool) Name() string        { return "TodoWrite" }
func (t *TodoTool) Description() string  { return "Write and update a todo list for tracking task progress" }

func (t *TodoTool) Execute(_ context.Context, input TodoInput) (*agent.ToolResult, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.todos = make([]TodoItem, len(input.Todos))
	copy(t.todos, input.Todos)

	pending, inProgress, completed := 0, 0, 0
	for _, item := range t.todos {
		switch item.Status {
		case "pending":
			pending++
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		}
	}

	return agent.TextResult(fmt.Sprintf("Todo list updated: %d pending, %d in progress, %d completed",
		pending, inProgress, completed)), nil
}

// Todos returns a snapshot of the current todo list.
func (t *TodoTool) Todos() []TodoItem {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]TodoItem, len(t.todos))
	copy(result, t.todos)
	return result
}
