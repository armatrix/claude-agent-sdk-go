package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// TaskCreateInput defines the input for the TaskCreate tool.
type TaskCreateInput struct {
	Subject     string `json:"subject" jsonschema:"required,description=Brief task title"`
	Description string `json:"description" jsonschema:"required,description=Task details including input files and done criteria"`
}

// TaskCreateTool creates a new task in the shared task list.
type TaskCreateTool struct {
	Tasks *teams.SharedTaskList
}

var _ agent.Tool[TaskCreateInput] = (*TaskCreateTool)(nil)

// NewTaskCreateTool creates a TaskCreateTool for the given task list.
func NewTaskCreateTool(tasks *teams.SharedTaskList) *TaskCreateTool {
	return &TaskCreateTool{Tasks: tasks}
}

func (t *TaskCreateTool) Name() string { return "TaskCreate" }
func (t *TaskCreateTool) Description() string {
	return "Create a new task in the shared task list"
}

func (t *TaskCreateTool) Execute(_ context.Context, input TaskCreateInput) (*agent.ToolResult, error) {
	if input.Subject == "" {
		return agent.ErrorResult("subject is required"), nil
	}
	if input.Description == "" {
		return agent.ErrorResult("description is required"), nil
	}

	task, err := t.Tasks.Create(input.Subject, input.Description)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to create task: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Task created: id=%s subject=%q", task.ID, task.Subject)), nil
}
