package teams

// Pipeline implements a unidirectional chain topology: A → B → C.
// Each stage passes its output to the next stage in order.
type Pipeline struct {
	Stages []string // ordered member names
}

func (t *Pipeline) Name() string { return "pipeline" }

func (t *Pipeline) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	for i, name := range t.Stages {
		if name == from && i < len(t.Stages)-1 {
			return []string{t.Stages[i+1]}
		}
	}
	return nil
}

func (t *Pipeline) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	// Pipeline stages process sequentially; no auto-assignment
	return nil
}

func (t *Pipeline) OnMemberJoin(name string)  {}
func (t *Pipeline) OnMemberLeave(name string) {}
