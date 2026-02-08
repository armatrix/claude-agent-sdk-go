package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/armatrix/claude-agent-sdk-go/teams"
)

func TestTaskCreateTool_Name(t *testing.T) {
	tool := &TaskCreateTool{}
	assert.Equal(t, "TaskCreate", tool.Name())
}

func TestTaskCreateTool_Description(t *testing.T) {
	tool := &TaskCreateTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestTaskCreateTool_Execute_Success(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskCreateTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskCreateInput{
		Subject:     "Build API",
		Description: "Implement REST endpoints for user module",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "Task created")
	assert.Contains(t, text, "Build API")

	// Verify task was actually created
	all := tasks.List(nil)
	assert.Len(t, all, 1)
	assert.Equal(t, "Build API", all[0].Subject)
	assert.Equal(t, "Implement REST endpoints for user module", all[0].Description)
}

func TestTaskCreateTool_Execute_EmptySubject(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskCreateTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskCreateInput{
		Subject:     "",
		Description: "desc",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "subject is required")
}

func TestTaskCreateTool_Execute_EmptyDescription(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskCreateTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskCreateInput{
		Subject:     "Task",
		Description: "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "description is required")
}

func TestTaskCreateTool_Execute_MultipleTasks(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskCreateTool{Tasks: tasks}

	_, err := tool.Execute(context.Background(), TaskCreateInput{
		Subject: "Task 1", Description: "First",
	})
	require.NoError(t, err)

	_, err = tool.Execute(context.Background(), TaskCreateInput{
		Subject: "Task 2", Description: "Second",
	})
	require.NoError(t, err)

	all := tasks.List(nil)
	assert.Len(t, all, 2)
}

func TestBroadcastTool_Name(t *testing.T) {
	tool := &BroadcastTool{}
	assert.Equal(t, "Broadcast", tool.Name())
}

func TestBroadcastTool_Execute_Success(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})
	bus.Subscribe("lead", 10)
	bus.Subscribe("alice", 10)
	bus.Subscribe("bob", 10)

	tool := &BroadcastTool{
		Bus:        bus,
		SenderName: "lead",
	}

	result, err := tool.Execute(context.Background(), BroadcastInput{
		Content: "attention everyone",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "2 members")
}

func TestBroadcastTool_Execute_EmptyContent(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})

	tool := &BroadcastTool{
		Bus:        bus,
		SenderName: "lead",
	}

	result, err := tool.Execute(context.Background(), BroadcastInput{Content: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "content is required")
}

func TestTaskListTool_Execute_Empty(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskListTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskListInput{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "No tasks found")
}

func TestTaskListTool_Execute_WithTasks(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tasks.Create("Task A", "desc a")
	tasks.Create("Task B", "desc b")
	tool := &TaskListTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskListInput{})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "2 total")
	assert.Contains(t, text, "Task A")
	assert.Contains(t, text, "Task B")
}

func TestTaskUpdateTool_Execute_UpdateStatus(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	task, _ := tasks.Create("Task", "desc")
	tool := &TaskUpdateTool{Tasks: tasks}

	status := "completed"
	result, err := tool.Execute(context.Background(), TaskUpdateInput{
		TaskID: task.ID,
		Status: &status,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	updated, _ := tasks.Get(task.ID)
	assert.Equal(t, teams.TaskCompleted, updated.Status)
}

func TestTaskUpdateTool_Execute_InvalidStatus(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	task, _ := tasks.Create("Task", "desc")
	tool := &TaskUpdateTool{Tasks: tasks}

	status := "invalid_status"
	result, err := tool.Execute(context.Background(), TaskUpdateInput{
		TaskID: task.ID,
		Status: &status,
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "invalid status")
}

func TestTaskUpdateTool_Execute_EmptyTaskID(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskUpdateTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskUpdateInput{TaskID: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "task_id is required")
}

func TestTaskGetTool_Execute_Success(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	task, _ := tasks.Create("My Task", "My description")
	tool := &TaskGetTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskGetInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, task.ID)
	assert.Contains(t, text, "My Task")
	assert.Contains(t, text, "My description")
	assert.Contains(t, text, "pending")
}

func TestTaskGetTool_Execute_NotFound(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskGetTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskGetInput{TaskID: "nope"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "not found")
}

func TestTaskGetTool_Execute_EmptyTaskID(t *testing.T) {
	tasks := teams.NewSharedTaskList()
	tool := &TaskGetTool{Tasks: tasks}

	result, err := tool.Execute(context.Background(), TaskGetInput{TaskID: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "task_id is required")
}

func TestShutdownRequestTool_Execute_Success(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})
	chWorker := bus.Subscribe("worker", 10)
	bus.Subscribe("lead", 10)

	tool := &ShutdownRequestTool{
		Bus:        bus,
		SenderName: "lead",
	}

	result, err := tool.Execute(context.Background(), ShutdownRequestInput{
		Recipient: "worker",
		Reason:    "task complete",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "Shutdown request sent to worker")

	// Verify the message arrived
	msg := <-chWorker
	assert.Equal(t, teams.MessageShutdownRequest, msg.Type)
	assert.Equal(t, "task complete", msg.Content)
	assert.NotEmpty(t, msg.RequestID)
}

func TestShutdownRequestTool_Execute_EmptyRecipient(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})
	tool := &ShutdownRequestTool{
		Bus:        bus,
		SenderName: "lead",
	}

	result, err := tool.Execute(context.Background(), ShutdownRequestInput{Recipient: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "recipient is required")
}

func TestShutdownRequestTool_Execute_DefaultReason(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})
	chWorker := bus.Subscribe("worker", 10)
	bus.Subscribe("lead", 10)

	tool := &ShutdownRequestTool{
		Bus:        bus,
		SenderName: "lead",
	}

	_, err := tool.Execute(context.Background(), ShutdownRequestInput{
		Recipient: "worker",
	})
	require.NoError(t, err)

	msg := <-chWorker
	assert.Equal(t, "shutdown requested", msg.Content)
}
