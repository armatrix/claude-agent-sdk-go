package tools

import (
	"context"
	"fmt"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// TaskListInput defines the input for the TaskList tool. No input is required.
type TaskListInput struct{}

// TaskListTool returns a summary of all tasks in the shared task list.
type TaskListTool struct {
	Tasks *teams.SharedTaskList
}

var _ agent.Tool[TaskListInput] = (*TaskListTool)(nil)

func (t *TaskListTool) Name() string { return "TaskList" }
func (t *TaskListTool) Description() string {
	return "List all tasks in the shared task list with their status"
}

func (t *TaskListTool) Execute(_ context.Context, _ TaskListInput) (*agent.ToolResult, error) {
	tasks := t.Tasks.List(nil)
	if len(tasks) == 0 {
		return agent.TextResult("No tasks found"), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Tasks (%d total):\n", len(tasks))
	for _, task := range tasks {
		status := taskStatusString(task.Status)
		owner := task.Owner
		if owner == "" {
			owner = "unassigned"
		}
		fmt.Fprintf(&b, "- [%s] %s (id=%s, owner=%s)\n", status, task.Subject, task.ID, owner)
	}
	return agent.TextResult(b.String()), nil
}

func taskStatusString(s teams.TaskStatus) string {
	switch s {
	case teams.TaskPending:
		return "pending"
	case teams.TaskInProgress:
		return "in_progress"
	case teams.TaskCompleted:
		return "completed"
	case teams.TaskDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}
