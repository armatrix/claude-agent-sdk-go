package tools

import (
	"context"
	"fmt"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// TaskGetInput defines the input for the TaskGet tool.
type TaskGetInput struct {
	TaskID string `json:"task_id" jsonschema:"required,description=Task ID to retrieve"`
}

// TaskGetTool retrieves details of a specific task from the shared task list.
type TaskGetTool struct {
	Tasks *teams.SharedTaskList
}

var _ agent.Tool[TaskGetInput] = (*TaskGetTool)(nil)

// NewTaskGetTool creates a TaskGetTool for the given task list.
func NewTaskGetTool(tasks *teams.SharedTaskList) *TaskGetTool {
	return &TaskGetTool{Tasks: tasks}
}

func (t *TaskGetTool) Name() string { return "TaskGet" }
func (t *TaskGetTool) Description() string {
	return "Get details of a specific task by ID"
}

func (t *TaskGetTool) Execute(_ context.Context, input TaskGetInput) (*agent.ToolResult, error) {
	if input.TaskID == "" {
		return agent.ErrorResult("task_id is required"), nil
	}

	task, err := t.Tasks.Get(input.TaskID)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("task not found: %s", err.Error())), nil
	}

	owner := task.Owner
	if owner == "" {
		owner = "unassigned"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Task: %s\n", task.ID)
	fmt.Fprintf(&b, "Subject: %s\n", task.Subject)
	fmt.Fprintf(&b, "Description: %s\n", task.Description)
	fmt.Fprintf(&b, "Status: %s\n", taskStatusString(task.Status))
	fmt.Fprintf(&b, "Owner: %s\n", owner)
	fmt.Fprintf(&b, "Created: %s\n", task.CreatedAt.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(&b, "Updated: %s\n", task.UpdatedAt.Format("2006-01-02T15:04:05Z"))

	if len(task.BlockedBy) > 0 {
		fmt.Fprintf(&b, "Blocked by: %s\n", strings.Join(task.BlockedBy, ", "))
	}
	if len(task.Blocks) > 0 {
		fmt.Fprintf(&b, "Blocks: %s\n", strings.Join(task.Blocks, ", "))
	}

	return agent.TextResult(b.String()), nil
}
