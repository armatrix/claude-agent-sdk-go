package teams

// SupervisorTree implements a hierarchical tree topology.
// Each member has a parent (supervisor) and optional children.
// Messages flow up to supervisor or down to children.
type SupervisorTree struct {
	Parent   map[string]string   // child → parent
	Children map[string][]string // parent → children
}

func (t *SupervisorTree) Name() string { return "supervisor-tree" }

func (t *SupervisorTree) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	// Default: route to parent (upward)
	if parent, ok := t.Parent[from]; ok {
		return []string{parent}
	}
	return nil
}

func (t *SupervisorTree) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	// Supervisors delegate to their children; no auto-assignment
	return nil
}

func (t *SupervisorTree) OnMemberJoin(name string)  {}
func (t *SupervisorTree) OnMemberLeave(name string) {}
