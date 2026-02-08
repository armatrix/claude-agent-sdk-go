package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// TaskUpdateInput defines the input for the TaskUpdate tool.
type TaskUpdateInput struct {
	TaskID  string  `json:"task_id" jsonschema:"required,description=Task ID to update"`
	Status  *string `json:"status,omitempty" jsonschema:"description=New status: pending|in_progress|completed|deleted"`
	Owner   *string `json:"owner,omitempty" jsonschema:"description=New owner name"`
	Subject *string `json:"subject,omitempty" jsonschema:"description=New subject"`
}

// TaskUpdateTool updates an existing task in the shared task list.
type TaskUpdateTool struct {
	Tasks *teams.SharedTaskList
}

var _ agent.Tool[TaskUpdateInput] = (*TaskUpdateTool)(nil)

func (t *TaskUpdateTool) Name() string { return "TaskUpdate" }
func (t *TaskUpdateTool) Description() string {
	return "Update an existing task's status, owner, or subject"
}

func (t *TaskUpdateTool) Execute(_ context.Context, input TaskUpdateInput) (*agent.ToolResult, error) {
	if input.TaskID == "" {
		return agent.ErrorResult("task_id is required"), nil
	}

	update := teams.TaskUpdate{
		Subject: input.Subject,
		Owner:   input.Owner,
	}

	if input.Status != nil {
		status, err := parseTaskStatus(*input.Status)
		if err != nil {
			return agent.ErrorResult(err.Error()), nil
		}
		update.Status = &status
	}

	if err := t.Tasks.Update(input.TaskID, update); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to update task: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Task %s updated", input.TaskID)), nil
}

func parseTaskStatus(s string) (teams.TaskStatus, error) {
	switch s {
	case "pending":
		return teams.TaskPending, nil
	case "in_progress":
		return teams.TaskInProgress, nil
	case "completed":
		return teams.TaskCompleted, nil
	case "deleted":
		return teams.TaskDeleted, nil
	default:
		return teams.TaskPending, fmt.Errorf("invalid status %q: must be pending|in_progress|completed|deleted", s)
	}
}
