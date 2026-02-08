package teams

import (
	"slices"
	"sync"
)

// MapReduceTopology implements a fan-out/fan-in topology.
// A Dispatcher distributes work to Workers, who send results to a Merger.
//
// NextTask distributes pending tasks evenly among idle workers.
// Dispatcher and Merger are coordination roles — they don't receive work tasks.
type MapReduceTopology struct {
	Dispatcher string   // distributes tasks
	Merger     string   // collects results
	Workers    []string // parallel processors

	mu            sync.Mutex
	activeWorkers []string // dynamic worker list
}

func (t *MapReduceTopology) Name() string { return "map-reduce" }

func (t *MapReduceTopology) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	workers := t.currentWorkers()
	// Dispatcher fans out to all workers
	if from == t.Dispatcher {
		return append([]string{}, workers...)
	}
	// Workers fan in to merger
	if slices.Contains(workers, from) {
		return []string{t.Merger}
	}
	// Merger sends to dispatcher (feedback loop)
	if from == t.Merger {
		return []string{t.Dispatcher}
	}
	return nil
}

// NextTask distributes pending, unblocked tasks evenly among idle workers.
// Dispatcher and Merger are excluded from work assignment.
func (t *MapReduceTopology) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	workers := t.currentWorkers()
	idleSet := idleMembers(members)

	// Only assign to idle workers
	var idleWorkers []string
	for _, w := range workers {
		if idleSet[w] {
			idleWorkers = append(idleWorkers, w)
		}
	}
	if len(idleWorkers) == 0 {
		return nil
	}

	var assignments []TaskAssignment
	wIdx := 0
	for _, task := range tasks {
		if wIdx >= len(idleWorkers) {
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
			MemberName: idleWorkers[wIdx],
		})
		wIdx++
	}
	return assignments
}

func (t *MapReduceTopology) OnMemberJoin(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, w := range t.Workers {
		if w == name {
			for _, a := range t.activeWorkers {
				if a == name {
					return
				}
			}
			t.activeWorkers = append(t.activeWorkers, name)
			return
		}
	}
}

func (t *MapReduceTopology) OnMemberLeave(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, w := range t.activeWorkers {
		if w == name {
			t.activeWorkers = append(t.activeWorkers[:i], t.activeWorkers[i+1:]...)
			return
		}
	}
}

func (t *MapReduceTopology) currentWorkers() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.activeWorkers) == 0 {
		t.activeWorkers = make([]string, len(t.Workers))
		copy(t.activeWorkers, t.Workers)
	}
	out := make([]string, len(t.activeWorkers))
	copy(out, t.activeWorkers)
	return out
}
