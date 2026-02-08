package teams

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessage(t *testing.T) {
	msg := NewMessage(MessageDM, "alice", "bob", "hello")
	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, MessageDM, msg.Type)
	assert.Equal(t, "alice", msg.From)
	assert.Equal(t, "bob", msg.To)
	assert.Equal(t, "hello", msg.Content)
	assert.False(t, msg.Timestamp.IsZero())
}

func TestMessageBus_Subscribe(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})

	ch := bus.Subscribe("alice", 10)
	assert.NotNil(t, ch)

	names := bus.MemberNames()
	assert.Contains(t, names, "alice")
}

func TestMessageBus_Unsubscribe(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})

	bus.Subscribe("alice", 10)
	bus.Unsubscribe("alice")

	names := bus.MemberNames()
	assert.NotContains(t, names, "alice")
}

func TestMessageBus_Send_DirectMessage(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})
	chAlice := bus.Subscribe("alice", 10)
	bus.Subscribe("bob", 10)

	msg := NewMessage(MessageDM, "bob", "alice", "hi alice")
	err := bus.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-chAlice:
		assert.Equal(t, "hi alice", received.Content)
		assert.Equal(t, "bob", received.From)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestMessageBus_Send_ViaTopology_LeaderTeammate(t *testing.T) {
	topo := &LeaderTeammate{LeaderName: "lead"}
	bus := NewMessageBus(topo)

	chLead := bus.Subscribe("lead", 10)
	bus.Subscribe("worker", 10)

	// Worker sends without explicit recipient — should route to leader
	msg := NewMessage(MessageDM, "worker", "", "status update")
	err := bus.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-chLead:
		assert.Equal(t, "status update", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestMessageBus_Send_ViaTopology_Pipeline(t *testing.T) {
	topo := &Pipeline{Stages: []string{"stage1", "stage2", "stage3"}}
	bus := NewMessageBus(topo)

	bus.Subscribe("stage1", 10)
	chStage2 := bus.Subscribe("stage2", 10)
	bus.Subscribe("stage3", 10)

	// stage1 sends without recipient — should go to stage2
	msg := NewMessage(MessageDM, "stage1", "", "processed data")
	err := bus.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-chStage2:
		assert.Equal(t, "processed data", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestMessageBus_Send_ViaTopology_PeerRing(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	bus := NewMessageBus(topo)

	bus.Subscribe("a", 10)
	chB := bus.Subscribe("b", 10)
	bus.Subscribe("c", 10)

	// a sends without recipient — should go to b (next in ring)
	msg := NewMessage(MessageDM, "a", "", "ring message")
	err := bus.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-chB:
		assert.Equal(t, "ring message", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestMessageBus_Send_ViaTopology_SupervisorTree(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"child": "parent"},
		Children: map[string][]string{"parent": {"child"}},
	}
	bus := NewMessageBus(topo)

	chParent := bus.Subscribe("parent", 10)
	bus.Subscribe("child", 10)

	// child sends without recipient — should go to parent
	msg := NewMessage(MessageDM, "child", "", "help")
	err := bus.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-chParent:
		assert.Equal(t, "help", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestMessageBus_Send_ViaTopology_MapReduce(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "dispatch",
		Merger:     "merge",
		Workers:    []string{"w1", "w2"},
	}
	bus := NewMessageBus(topo)

	bus.Subscribe("dispatch", 10)
	chW1 := bus.Subscribe("w1", 10)
	chW2 := bus.Subscribe("w2", 10)
	bus.Subscribe("merge", 10)

	// Dispatcher fans out to workers
	msg := NewMessage(MessageDM, "dispatch", "", "compute this")
	err := bus.Send(msg)
	require.NoError(t, err)

	select {
	case received := <-chW1:
		assert.Equal(t, "compute this", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for w1 message")
	}

	select {
	case received := <-chW2:
		assert.Equal(t, "compute this", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for w2 message")
	}
}

func TestMessageBus_Send_UnknownRecipient(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})
	bus.Subscribe("lead", 10)

	msg := NewMessage(MessageDM, "lead", "nobody", "hello?")
	err := bus.Send(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMessageBus_Broadcast(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})

	chLead := bus.Subscribe("lead", 10)
	chA := bus.Subscribe("alice", 10)
	chB := bus.Subscribe("bob", 10)

	msg := NewMessage(MessageBroadcast, "lead", "", "attention everyone")
	err := bus.Broadcast(msg)
	require.NoError(t, err)

	// Alice and Bob should receive
	select {
	case received := <-chA:
		assert.Equal(t, "attention everyone", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout for alice")
	}

	select {
	case received := <-chB:
		assert.Equal(t, "attention everyone", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout for bob")
	}

	// Lead (sender) should NOT receive
	select {
	case <-chLead:
		t.Fatal("leader should not receive its own broadcast")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestMessageBus_Broadcast_SkipsSender(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})

	chSender := bus.Subscribe("sender", 10)
	chReceiver := bus.Subscribe("receiver", 10)

	msg := NewMessage(MessageBroadcast, "sender", "", "hello all")
	err := bus.Broadcast(msg)
	require.NoError(t, err)

	// Receiver gets it
	select {
	case received := <-chReceiver:
		assert.Equal(t, "hello all", received.Content)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

	// Sender does not
	select {
	case <-chSender:
		t.Fatal("sender should not get own broadcast")
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}

func TestMessageBus_MemberNames(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})

	bus.Subscribe("alice", 10)
	bus.Subscribe("bob", 10)
	bus.Subscribe("charlie", 10)

	names := bus.MemberNames()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "alice")
	assert.Contains(t, names, "bob")
	assert.Contains(t, names, "charlie")
}

func TestMessageBus_UnsubscribeAndResubscribe(t *testing.T) {
	bus := NewMessageBus(&LeaderTeammate{LeaderName: "lead"})

	bus.Subscribe("alice", 10)
	bus.Unsubscribe("alice")

	// Should be able to resubscribe
	ch := bus.Subscribe("alice", 10)
	assert.NotNil(t, ch)
	assert.Contains(t, bus.MemberNames(), "alice")
}
