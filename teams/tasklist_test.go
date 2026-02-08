package teams

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedTaskList_Create(t *testing.T) {
	tl := NewSharedTaskList()
	task, err := tl.Create("Build API", "Implement REST endpoints")
	require.NoError(t, err)
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Build API", task.Subject)
	assert.Equal(t, "Implement REST endpoints", task.Description)
	assert.Equal(t, TaskPending, task.Status)
	assert.Empty(t, task.Owner)
	assert.False(t, task.CreatedAt.IsZero())
	assert.False(t, task.UpdatedAt.IsZero())
}

func TestSharedTaskList_Get(t *testing.T) {
	tl := NewSharedTaskList()
	created, _ := tl.Create("Task A", "Description A")

	task, err := tl.Get(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, task.ID)
	assert.Equal(t, "Task A", task.Subject)
}

func TestSharedTaskList_Get_NotFound(t *testing.T) {
	tl := NewSharedTaskList()
	_, err := tl.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSharedTaskList_Update_Subject(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Old Subject", "Description")

	newSubject := "New Subject"
	err := tl.Update(task.ID, TaskUpdate{Subject: &newSubject})
	require.NoError(t, err)

	updated, _ := tl.Get(task.ID)
	assert.Equal(t, "New Subject", updated.Subject)
}

func TestSharedTaskList_Update_Status(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Task", "Desc")

	status := TaskInProgress
	err := tl.Update(task.ID, TaskUpdate{Status: &status})
	require.NoError(t, err)

	updated, _ := tl.Get(task.ID)
	assert.Equal(t, TaskInProgress, updated.Status)
}

func TestSharedTaskList_Update_Owner(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Task", "Desc")

	owner := "alice"
	err := tl.Update(task.ID, TaskUpdate{Owner: &owner})
	require.NoError(t, err)

	updated, _ := tl.Get(task.ID)
	assert.Equal(t, "alice", updated.Owner)
}

func TestSharedTaskList_Update_NotFound(t *testing.T) {
	tl := NewSharedTaskList()
	err := tl.Update("nope", TaskUpdate{})
	assert.Error(t, err)
}

func TestSharedTaskList_Claim(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Task", "Desc")

	err := tl.Claim(task.ID, "alice")
	require.NoError(t, err)

	claimed, _ := tl.Get(task.ID)
	assert.Equal(t, "alice", claimed.Owner)
	assert.Equal(t, TaskInProgress, claimed.Status)
}

func TestSharedTaskList_Claim_NotPending(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Task", "Desc")

	status := TaskCompleted
	_ = tl.Update(task.ID, TaskUpdate{Status: &status})

	err := tl.Claim(task.ID, "bob")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestSharedTaskList_Claim_AlreadyOwned(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Task", "Desc")

	owner := "alice"
	_ = tl.Update(task.ID, TaskUpdate{Owner: &owner})

	err := tl.Claim(task.ID, "bob")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already owned")
}

func TestSharedTaskList_Claim_Blocked(t *testing.T) {
	tl := NewSharedTaskList()
	blocker, _ := tl.Create("Blocker", "Must finish first")
	task, _ := tl.Create("Task", "Depends on blocker")

	// Add blocker dependency
	err := tl.Update(task.ID, TaskUpdate{AddBlockedBy: []string{blocker.ID}})
	require.NoError(t, err)

	// Claim should fail because blocker is not completed
	err = tl.Claim(task.ID, "alice")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked by")
}

func TestSharedTaskList_Claim_BlockerCompleted(t *testing.T) {
	tl := NewSharedTaskList()
	blocker, _ := tl.Create("Blocker", "Must finish first")
	task, _ := tl.Create("Task", "Depends on blocker")

	// Add dependency
	_ = tl.Update(task.ID, TaskUpdate{AddBlockedBy: []string{blocker.ID}})

	// Complete the blocker
	status := TaskCompleted
	_ = tl.Update(blocker.ID, TaskUpdate{Status: &status})

	// Now claim should succeed
	err := tl.Claim(task.ID, "alice")
	require.NoError(t, err)
}

func TestSharedTaskList_NextAvailable(t *testing.T) {
	tl := NewSharedTaskList()
	task1, _ := tl.Create("First", "First task")
	tl.Create("Second", "Second task")

	next := tl.NextAvailable()
	assert.NotNil(t, next)
	assert.Equal(t, task1.ID, next.ID)
}

func TestSharedTaskList_NextAvailable_SkipsOwned(t *testing.T) {
	tl := NewSharedTaskList()
	task1, _ := tl.Create("First", "Owned")
	task2, _ := tl.Create("Second", "Free")

	// Own the first task
	owner := "alice"
	_ = tl.Update(task1.ID, TaskUpdate{Owner: &owner})

	next := tl.NextAvailable()
	assert.NotNil(t, next)
	assert.Equal(t, task2.ID, next.ID)
}

func TestSharedTaskList_NextAvailable_SkipsBlocked(t *testing.T) {
	tl := NewSharedTaskList()
	blocker, _ := tl.Create("Blocker", "Blocks task2")
	task2, _ := tl.Create("Task2", "Blocked")
	task3, _ := tl.Create("Task3", "Free")

	_ = tl.Update(task2.ID, TaskUpdate{AddBlockedBy: []string{blocker.ID}})

	next := tl.NextAvailable()
	// blocker is first and available
	assert.Equal(t, blocker.ID, next.ID)

	// Claim the blocker
	_ = tl.Claim(blocker.ID, "alice")

	next = tl.NextAvailable()
	// task2 is blocked, task3 is free
	assert.Equal(t, task3.ID, next.ID)
}

func TestSharedTaskList_NextAvailable_NoneAvailable(t *testing.T) {
	tl := NewSharedTaskList()
	task, _ := tl.Create("Only", "The only task")
	_ = tl.Claim(task.ID, "alice")

	next := tl.NextAvailable()
	assert.Nil(t, next)
}

func TestSharedTaskList_List_NoFilter(t *testing.T) {
	tl := NewSharedTaskList()
	tl.Create("A", "a")
	tl.Create("B", "b")
	tl.Create("C", "c")

	tasks := tl.List(nil)
	assert.Len(t, tasks, 3)
}

func TestSharedTaskList_List_FilterByStatus(t *testing.T) {
	tl := NewSharedTaskList()
	task1, _ := tl.Create("Pending", "p")
	tl.Create("Also Pending", "p2")

	_ = tl.Claim(task1.ID, "alice")

	status := TaskInProgress
	tasks := tl.List(&TaskFilter{Status: &status})
	assert.Len(t, tasks, 1)
	assert.Equal(t, task1.ID, tasks[0].ID)
}

func TestSharedTaskList_List_FilterByOwner(t *testing.T) {
	tl := NewSharedTaskList()
	task1, _ := tl.Create("Task1", "d1")
	tl.Create("Task2", "d2")

	owner := "alice"
	_ = tl.Update(task1.ID, TaskUpdate{Owner: &owner})

	tasks := tl.List(&TaskFilter{Owner: &owner})
	assert.Len(t, tasks, 1)
	assert.Equal(t, "alice", tasks[0].Owner)
}

func TestSharedTaskList_List_ExcludesDeleted(t *testing.T) {
	tl := NewSharedTaskList()
	task1, _ := tl.Create("Active", "keep")
	task2, _ := tl.Create("Deleted", "remove")

	status := TaskDeleted
	_ = tl.Update(task2.ID, TaskUpdate{Status: &status})
	_ = task1 // suppress unused

	tasks := tl.List(nil)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "Active", tasks[0].Subject)
}
