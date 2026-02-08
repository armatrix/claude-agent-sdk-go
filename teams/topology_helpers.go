package teams

// idleMembers returns a set of member names that are currently idle.
func idleMembers(members []*Member) map[string]bool {
	idle := make(map[string]bool, len(members))
	for _, m := range members {
		if m.Status() == MemberIdle {
			idle[m.Name()] = true
		}
	}
	return idle
}

// isBlocked returns true if any of the task's blockers are not yet completed.
func isBlocked(task *Task, allTasks []*Task) bool {
	if len(task.BlockedBy) == 0 {
		return false
	}
	taskMap := make(map[string]*Task, len(allTasks))
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}
	for _, blockerID := range task.BlockedBy {
		if blocker, ok := taskMap[blockerID]; ok && blocker.Status != TaskCompleted {
			return true
		}
	}
	return false
}
