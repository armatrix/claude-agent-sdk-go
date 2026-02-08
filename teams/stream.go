package teams

import agent "github.com/armatrix/claude-agent-sdk-go"

// Event wraps an agent event with the member name that produced it.
type Event struct {
	MemberName string
	AgentEvent agent.Event
}

// Stream aggregates events from all team members into a single iterator.
type Stream struct {
	events  chan *Event
	current *Event
	err     error
	done    bool
}

// Next advances to the next event. Returns false when done or on error.
func (s *Stream) Next() bool {
	if s.done {
		return false
	}
	ev, ok := <-s.events
	if !ok {
		s.done = true
		return false
	}
	s.current = ev
	return true
}

// Current returns the most recently read event.
func (s *Stream) Current() *Event { return s.current }

// Err returns the first error encountered during streaming.
func (s *Stream) Err() error { return s.err }
