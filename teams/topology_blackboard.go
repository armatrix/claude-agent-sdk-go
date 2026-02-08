package teams

// BlackboardTopology implements indirect communication via a shared Blackboard.
// Agents read/write to the blackboard rather than sending direct messages.
// The Route method returns no targets; communication happens through the board.
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

func (t *BlackboardTopology) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	// Agents self-select based on blackboard state; no auto-assignment
	return nil
}

func (t *BlackboardTopology) OnMemberJoin(name string)  {}
func (t *BlackboardTopology) OnMemberLeave(name string) {}
