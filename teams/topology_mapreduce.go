package teams

import "slices"

// MapReduceTopology implements a fan-out/fan-in topology.
// A Dispatcher distributes work to Workers, who send results to a Merger.
type MapReduceTopology struct {
	Dispatcher string   // distributes tasks
	Merger     string   // collects results
	Workers    []string // parallel processors
}

func (t *MapReduceTopology) Name() string { return "map-reduce" }

func (t *MapReduceTopology) Route(from string, msg *Message, members []string) []string {
	if msg.To != "" {
		return []string{msg.To}
	}
	// Dispatcher fans out to all workers
	if from == t.Dispatcher {
		return append([]string{}, t.Workers...)
	}
	// Workers fan in to merger
	if slices.Contains(t.Workers, from) {
		return []string{t.Merger}
	}
	// Merger sends to dispatcher (feedback loop)
	if from == t.Merger {
		return []string{t.Dispatcher}
	}
	return nil
}

func (t *MapReduceTopology) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	// Dispatcher handles task distribution; no auto-assignment
	return nil
}

func (t *MapReduceTopology) OnMemberJoin(name string)  {}
func (t *MapReduceTopology) OnMemberLeave(name string) {}
