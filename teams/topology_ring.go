package teams

import "sync"

// PeerRing implements a fully-connected ring topology.
// Each member can send to any other; default routing goes to the next in ring.
// Tasks are assigned round-robin among idle ring members.
type PeerRing struct {
	Ring []string // ordered member names forming the ring

	mu          sync.Mutex
	activeRing  []string // dynamic ring, updated on join/leave
	lastAssign  int      // round-robin index for NextTask
}

func (t *PeerRing) Name() string { return "peer-ring" }

func (t *PeerRing) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	ring := t.currentRing()
	for i, name := range ring {
		if name == from {
			next := (i + 1) % len(ring)
			return []string{ring[next]}
		}
	}
	return nil
}

// NextTask assigns pending, unblocked tasks round-robin to idle ring members.
func (t *PeerRing) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	ring := t.currentRing()
	if len(ring) == 0 {
		return nil
	}
	idleSet := idleMembers(members)

	var assignments []TaskAssignment
	assigned := make(map[string]bool)

	for _, task := range tasks {
		if task.Status != TaskPending || task.Owner != "" {
			continue
		}
		if isBlocked(task, tasks) {
			continue
		}
		// Find next idle ring member via round-robin
		memberName := t.nextIdle(ring, idleSet, assigned)
		if memberName == "" {
			break // no idle members left
		}
		assignments = append(assignments, TaskAssignment{
			TaskID:     task.ID,
			MemberName: memberName,
		})
		assigned[memberName] = true
	}
	return assignments
}

func (t *PeerRing) OnMemberJoin(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Add to ring if it was originally configured
	for _, r := range t.Ring {
		if r == name {
			for _, a := range t.activeRing {
				if a == name {
					return
				}
			}
			t.activeRing = append(t.activeRing, name)
			return
		}
	}
}

func (t *PeerRing) OnMemberLeave(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, r := range t.activeRing {
		if r == name {
			t.activeRing = append(t.activeRing[:i], t.activeRing[i+1:]...)
			return
		}
	}
}

func (t *PeerRing) currentRing() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.activeRing) == 0 {
		t.activeRing = make([]string, len(t.Ring))
		copy(t.activeRing, t.Ring)
	}
	out := make([]string, len(t.activeRing))
	copy(out, t.activeRing)
	return out
}

// nextIdle finds the next idle ring member in round-robin order.
func (t *PeerRing) nextIdle(ring []string, idleSet map[string]bool, assigned map[string]bool) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for range ring {
		idx := t.lastAssign % len(ring)
		t.lastAssign++
		name := ring[idx]
		if idleSet[name] && !assigned[name] {
			return name
		}
	}
	return ""
}
