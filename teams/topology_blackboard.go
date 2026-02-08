package teams

// BlackboardTopology implements indirect communication via a shared Blackboard.
// Agents read/write to the blackboard rather than sending direct messages.
// The Route method returns no targets; communication happens through the board.
//
// NextTask assigns pending tasks to idle members. In a real scenario, agents
// watch the board for relevant entries and self-select work. The auto-assignment
// here serves as a fallback to ensure progress.
type BlackboardTopology struct {
	Board *Blackboard
}

func (t *BlackboardTopology) Name() string { return "blackboard" }

func (t *BlackboardTopology) Route(from string, msg *Message, members []string) []string {
	// Blackboard topology: no direct routing.
	// Agents communicate by writing to and reading from the Blackboard.
	if msg.To != "" {
		return []string{msg.To} // allow explicit DMs as escape hatch
	}
	return nil
}

// NextTask assigns pending, unblocked tasks to idle members. Blackboard agents
// are loosely coupled — any idle member can pick up any task.
func (t *BlackboardTopology) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	idleSet := idleMembers(members)

	var idle []string
	for _, m := range members {
		if idleSet[m.Name()] {
			idle = append(idle, m.Name())
		}
	}

	var assignments []TaskAssignment
	idx := 0
	for _, task := range tasks {
		if idx >= len(idle) {
			break
		}
		if task.Status != TaskPending || task.Owner != "" {
			continue
		}
		if isBlocked(task, tasks) {
			continue
		}
		assignments = append(assignments, TaskAssignment{
			TaskID:     task.ID,
			MemberName: idle[idx],
		})
		idx++
	}
	return assignments
}

func (t *BlackboardTopology) OnMemberJoin(name string)  {}
func (t *BlackboardTopology) OnMemberLeave(name string) {}
