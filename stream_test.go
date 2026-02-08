package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyStream(t *testing.T) {
	stream := emptyStream()

	assert.False(t, stream.Next(), "empty stream should return false on first Next()")
	assert.Nil(t, stream.Current(), "current should be nil on empty stream")
	assert.NoError(t, stream.Err(), "empty stream should have no error")
	require.NotNil(t, stream.Session(), "session should not be nil")
}

func TestStreamIteratesEvents(t *testing.T) {
	ch := make(chan Event, 3)
	session := NewSession()
	stream := newStream(ch, session)

	ch <- &SystemEvent{SessionID: "s1", Model: DefaultModel}
	ch <- &StreamEvent{Delta: "hello"}
	ch <- &AssistantEvent{}
	close(ch)

	// Event 1: SystemEvent
	assert.True(t, stream.Next())
	ev1, ok := stream.Current().(*SystemEvent)
	require.True(t, ok, "expected SystemEvent")
	assert.Equal(t, "s1", ev1.SessionID)
	assert.Equal(t, EventSystem, ev1.Type())

	// Event 2: StreamEvent
	assert.True(t, stream.Next())
	ev2, ok := stream.Current().(*StreamEvent)
	require.True(t, ok, "expected StreamEvent")
	assert.Equal(t, "hello", ev2.Delta)
	assert.Equal(t, EventStream, ev2.Type())

	// Event 3: AssistantEvent
	assert.True(t, stream.Next())
	_, ok = stream.Current().(*AssistantEvent)
	require.True(t, ok, "expected AssistantEvent")

	// Done
	assert.False(t, stream.Next())
	assert.NoError(t, stream.Err())
}

func TestStreamSessionReturned(t *testing.T) {
	session := NewSession()
	ch := make(chan Event)
	close(ch)
	stream := newStream(ch, session)

	assert.Equal(t, session, stream.Session())
}

func TestStreamNextAfterDone(t *testing.T) {
	stream := emptyStream()

	assert.False(t, stream.Next())
	assert.False(t, stream.Next(), "calling Next after done should still return false")
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		event    Event
		expected EventType
	}{
		{&SystemEvent{}, EventSystem},
		{&AssistantEvent{}, EventAssistant},
		{&StreamEvent{}, EventStream},
		{&ResultEvent{}, EventResult},
		{&CompactEvent{}, EventCompact},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, tc.event.Type(), "event type mismatch for %T", tc.event)
	}
}
