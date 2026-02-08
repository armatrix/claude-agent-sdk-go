package teams

import (
	"fmt"
	"sync"
	"time"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus int

const (
	TaskPending    TaskStatus = iota
	TaskInProgress
	TaskCompleted
	TaskDeleted
)

// Task is a unit of work in the shared task list.
type Task struct {
	ID          string
	Subject     string
	Description string
	Status      TaskStatus
	Owner       string // member name
	BlockedBy   []string
	Blocks      []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TaskUpdate holds optional fields for updating a task.
type TaskUpdate struct {
	Subject     *string
	Description *string
	Status      *TaskStatus
	Owner       *string
	AddBlockedBy []string
	AddBlocks    []string
}

// TaskFilter selects tasks by criteria.
type TaskFilter struct {
	Status *TaskStatus
	Owner  *string
}

// SharedTaskList is a concurrent-safe task list shared by all team members.
type SharedTaskList struct {
	tasks map[string]*Task
	order []string
	mu    sync.RWMutex
}

// NewSharedTaskList creates an empty task list.
func NewSharedTaskList() *SharedTaskList {
	return &SharedTaskList{
		tasks: make(map[string]*Task),
	}
}

// Create adds a new task to the list.
func (l *SharedTaskList) Create(subject, description string) (*Task, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	t := &Task{
		ID:          agent.GenerateID("task"),
		Subject:     subject,
		Description: description,
		Status:      TaskPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	l.tasks[t.ID] = t
	l.order = append(l.order, t.ID)
	return t, nil
}

// Get returns a task by ID.
func (l *SharedTaskList) Get(id string) (*Task, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	t, ok := l.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return t, nil
}

// Update modifies a task with the given updates.
func (l *SharedTaskList) Update(id string, u TaskUpdate) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	t, ok := l.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	if u.Subject != nil {
		t.Subject = *u.Subject
	}
	if u.Description != nil {
		t.Description = *u.Description
	}
	if u.Status != nil {
		t.Status = *u.Status
	}
	if u.Owner != nil {
		t.Owner = *u.Owner
	}
	t.BlockedBy = append(t.BlockedBy, u.AddBlockedBy...)
	t.Blocks = append(t.Blocks, u.AddBlocks...)
	t.UpdatedAt = time.Now()
	return nil
}

// Claim atomically assigns a pending, unblocked task to the given owner.
func (l *SharedTaskList) Claim(id, owner string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	t, ok := l.tasks[id]
	if !ok {
		return fmt.Errorf("task %q not found", id)
	}
	if t.Status != TaskPending {
		return fmt.Errorf("task %q is not pending (status=%d)", id, t.Status)
	}
	if t.Owner != "" {
		return fmt.Errorf("task %q already owned by %q", id, t.Owner)
	}
	// Check blockers
	for _, blockerID := range t.BlockedBy {
		if blocker, exists := l.tasks[blockerID]; exists && blocker.Status != TaskCompleted {
			return fmt.Errorf("task %q is blocked by %q", id, blockerID)
		}
	}
	t.Owner = owner
	t.Status = TaskInProgress
	t.UpdatedAt = time.Now()
	return nil
}

// NextAvailable returns the first pending, unblocked, unowned task.
func (l *SharedTaskList) NextAvailable() *Task {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, id := range l.order {
		t := l.tasks[id]
		if t.Status != TaskPending || t.Owner != "" {
			continue
		}
		blocked := false
		for _, blockerID := range t.BlockedBy {
			if blocker, exists := l.tasks[blockerID]; exists && blocker.Status != TaskCompleted {
				blocked = true
				break
			}
		}
		if !blocked {
			return t
		}
	}
	return nil
}

// List returns tasks matching the filter. Nil filter returns all.
func (l *SharedTaskList) List(filter *TaskFilter) []*Task {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]*Task, 0, len(l.order))
	for _, id := range l.order {
		t := l.tasks[id]
		if t.Status == TaskDeleted {
			continue
		}
		if filter != nil {
			if filter.Status != nil && t.Status != *filter.Status {
				continue
			}
			if filter.Owner != nil && t.Owner != *filter.Owner {
				continue
			}
		}
		result = append(result, t)
	}
	return result
}
