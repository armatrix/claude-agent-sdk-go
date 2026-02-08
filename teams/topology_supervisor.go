package teams

import "sync"

// SupervisorTree implements a hierarchical tree topology.
// Each member has a parent (supervisor) and optional children.
// Messages default to flowing upward (child → parent).
// Tasks are assigned to idle leaf workers; supervisors delegate, not execute.
type SupervisorTree struct {
	Parent   map[string]string   // child → parent
	Children map[string][]string // parent → children

	mu sync.Mutex
}

func (t *SupervisorTree) Name() string { return "supervisor-tree" }

func (t *SupervisorTree) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	t.mu.Lock()
	parent := t.Parent[from]
	t.mu.Unlock()
	if parent != "" {
		return []string{parent}
	}
	return nil
}

// NextTask assigns pending, unblocked tasks to idle leaf nodes (members with
// no children). Supervisors coordinate but don't receive work assignments.
func (t *SupervisorTree) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	t.mu.Lock()
	children := t.Children
	t.mu.Unlock()

	idleSet := idleMembers(members)

	// Collect leaf nodes: members that have no children
	var leaves []string
	for _, m := range members {
		name := m.Name()
		ch := children[name]
		if len(ch) == 0 && idleSet[name] {
			leaves = append(leaves, name)
		}
	}

	var assignments []TaskAssignment
	leafIdx := 0
	for _, task := range tasks {
		if leafIdx >= len(leaves) {
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
			MemberName: leaves[leafIdx],
		})
		leafIdx++
	}
	return assignments
}

func (t *SupervisorTree) OnMemberJoin(name string) {
	// Join notifications are informational; tree structure is set at creation.
}

// OnMemberLeave removes the member from the tree and re-parents orphaned
// children to the departing member's parent.
func (t *SupervisorTree) OnMemberLeave(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	parent := t.Parent[name]
	delete(t.Parent, name)

	// Re-parent orphaned children to the departed member's parent
	orphans := t.Children[name]
	delete(t.Children, name)
	for _, child := range orphans {
		t.Parent[child] = parent
		if parent != "" {
			t.Children[parent] = append(t.Children[parent], child)
		}
	}

	// Remove from parent's children list
	if parent != "" {
		siblings := t.Children[parent]
		for i, s := range siblings {
			if s == name {
				t.Children[parent] = append(siblings[:i], siblings[i+1:]...)
				break
			}
		}
	}
}
