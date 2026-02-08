package teams

import "sync"

// Pipeline implements a unidirectional chain topology: A → B → C.
// Each stage passes its output to the next stage in order.
// Tasks are assigned to idle members at the earliest stage with pending work.
type Pipeline struct {
	Stages []string // ordered member names

	mu           sync.Mutex
	stageMembers []string // dynamic copy, updated on join/leave
}

func (t *Pipeline) Name() string { return "pipeline" }

func (t *Pipeline) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	stages := t.activeStages()
	for i, name := range stages {
		if name == from && i < len(stages)-1 {
			return []string{stages[i+1]}
		}
	}
	return nil
}

// NextTask assigns pending, unblocked tasks to idle members at the earliest
// stage. Earlier stages get priority — this enforces sequential processing.
func (t *Pipeline) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	stages := t.activeStages()
	idleSet := idleMembers(members)

	var assignments []TaskAssignment
	assignedMembers := make(map[string]bool)
	assignedTasks := make(map[string]bool)

	// Walk stages front-to-back; assign pending tasks to idle stage members.
	for _, stageName := range stages {
		if !idleSet[stageName] || assignedMembers[stageName] {
			continue
		}
		for _, task := range tasks {
			if assignedTasks[task.ID] {
				continue
			}
			if task.Status != TaskPending || task.Owner != "" {
				continue
			}
			if isBlocked(task, tasks) {
				continue
			}
			assignments = append(assignments, TaskAssignment{
				TaskID:     task.ID,
				MemberName: stageName,
			})
			assignedMembers[stageName] = true
			assignedTasks[task.ID] = true
			break // one task per member
		}
	}
	return assignments
}

func (t *Pipeline) OnMemberJoin(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Add to stage list if it was originally in Stages
	for _, s := range t.Stages {
		if s == name {
			// Re-add to active list if not already present
			for _, a := range t.stageMembers {
				if a == name {
					return
				}
			}
			t.stageMembers = append(t.stageMembers, name)
			return
		}
	}
}

func (t *Pipeline) OnMemberLeave(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, s := range t.stageMembers {
		if s == name {
			t.stageMembers = append(t.stageMembers[:i], t.stageMembers[i+1:]...)
			return
		}
	}
}

// activeStages returns the current stage list (falling back to Stages if none set).
func (t *Pipeline) activeStages() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.stageMembers) == 0 {
		// Initialize from Stages
		t.stageMembers = make([]string, len(t.Stages))
		copy(t.stageMembers, t.Stages)
	}
	out := make([]string, len(t.stageMembers))
	copy(out, t.stageMembers)
	return out
}
