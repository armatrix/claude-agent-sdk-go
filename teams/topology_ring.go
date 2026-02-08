package teams

// PeerRing implements a fully-connected ring topology.
// Each member can send to any other; default routing goes to the next in ring.
type PeerRing struct {
	Ring []string // ordered member names forming the ring
}

func (t *PeerRing) Name() string { return "peer-ring" }

func (t *PeerRing) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	for i, name := range t.Ring {
		if name == from {
			next := (i + 1) % len(t.Ring)
			return []string{t.Ring[next]}
		}
	}
	return nil
}

func (t *PeerRing) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	// Peers self-coordinate; no auto-assignment
	return nil
}

func (t *PeerRing) OnMemberJoin(name string)  {}
func (t *PeerRing) OnMemberLeave(name string) {}
