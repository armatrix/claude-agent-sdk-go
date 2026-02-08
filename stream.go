package agent

// AgentStream is an iterator over events emitted during an agent run.
// Usage:
//
//	stream := agent.Run(ctx, "prompt")
//	for stream.Next() {
//	    event := stream.Current()
//	    // handle event
//	}
//	if err := stream.Err(); err != nil {
//	    // handle error
//	}
type AgentStream struct {
	events  chan Event
	current Event
	err     error
	done    bool
	session *Session
}

// newStream creates a new AgentStream with the given event channel and session.
func newStream(events chan Event, session *Session) *AgentStream {
	return &AgentStream{
		events:  events,
		session: session,
	}
}

// Next advances to the next event. Returns false when the stream is exhausted
// or an error has occurred.
func (s *AgentStream) Next() bool {
	if s.done {
		return false
	}
	event, ok := <-s.events
	if !ok {
		s.done = true
		return false
	}
	s.current = event
	return true
}

// Current returns the most recent event returned by Next.
func (s *AgentStream) Current() Event {
	return s.current
}

// Err returns the first error encountered during iteration, if any.
func (s *AgentStream) Err() error {
	return s.err
}

// Session returns the session associated with this stream.
// The session is populated with conversation history after the run completes.
func (s *AgentStream) Session() *Session {
	return s.session
}

// emptyStream returns a stream that immediately reports done (for stubs).
func emptyStream() *AgentStream {
	ch := make(chan Event)
	close(ch)
	return newStream(ch, NewSession())
}
