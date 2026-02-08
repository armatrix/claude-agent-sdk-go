package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoTool_Name(t *testing.T) {
	tool := &TodoTool{}
	assert.Equal(t, "TodoWrite", tool.Name())
}

func TestTodoTool_Description(t *testing.T) {
	tool := &TodoTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestTodoTool_Execute_WriteTodos(t *testing.T) {
	tool := &TodoTool{}
	input := TodoInput{
		Todos: []TodoItem{
			{ID: "1", Content: "Setup project", Status: "completed"},
			{ID: "2", Content: "Write tests", Status: "in_progress"},
			{ID: "3", Content: "Deploy", Status: "pending"},
			{ID: "4", Content: "Monitor", Status: "pending"},
		},
	}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "2 pending")
	assert.Contains(t, text, "1 in progress")
	assert.Contains(t, text, "1 completed")
}

func TestTodoTool_Execute_UpdateOverwrite(t *testing.T) {
	tool := &TodoTool{}

	// First write
	input1 := TodoInput{
		Todos: []TodoItem{
			{ID: "1", Content: "Task A", Status: "pending"},
		},
	}
	_, err := tool.Execute(context.Background(), input1)
	require.NoError(t, err)
	assert.Len(t, tool.Todos(), 1)

	// Overwrite with new list
	input2 := TodoInput{
		Todos: []TodoItem{
			{ID: "1", Content: "Task A", Status: "completed"},
			{ID: "2", Content: "Task B", Status: "in_progress"},
		},
	}
	result, err := tool.Execute(context.Background(), input2)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "0 pending")
	assert.Contains(t, text, "1 in progress")
	assert.Contains(t, text, "1 completed")

	todos := tool.Todos()
	assert.Len(t, todos, 2)
	assert.Equal(t, "completed", todos[0].Status)
}

func TestTodoTool_Execute_EmptyList(t *testing.T) {
	tool := &TodoTool{}
	input := TodoInput{Todos: []TodoItem{}}

	result, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "0 pending")
	assert.Contains(t, text, "0 in progress")
	assert.Contains(t, text, "0 completed")
}

func TestTodoTool_ConcurrentAccess(t *testing.T) {
	tool := &TodoTool{}
	var wg sync.WaitGroup

	// Run concurrent writes and reads
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			input := TodoInput{
				Todos: []TodoItem{
					{ID: "1", Content: "Task", Status: "pending"},
				},
			}
			_, err := tool.Execute(context.Background(), input)
			assert.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			_ = tool.Todos()
		}()
	}
	wg.Wait()
}

func TestTodoTool_Todos_ReturnsSnapshot(t *testing.T) {
	tool := &TodoTool{}
	input := TodoInput{
		Todos: []TodoItem{
			{ID: "1", Content: "Original", Status: "pending"},
		},
	}
	_, err := tool.Execute(context.Background(), input)
	require.NoError(t, err)

	// Get snapshot
	snapshot := tool.Todos()
	require.Len(t, snapshot, 1)
	assert.Equal(t, "Original", snapshot[0].Content)

	// Mutate snapshot
	snapshot[0].Content = "Modified"

	// Verify internal state is unchanged
	fresh := tool.Todos()
	assert.Equal(t, "Original", fresh[0].Content)
}
