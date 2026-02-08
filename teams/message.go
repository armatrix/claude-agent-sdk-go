package teams

import (
	"fmt"
	"sync"
	"time"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// MessageType identifies the kind of inter-agent message.
type MessageType string

const (
	MessageDM               MessageType = "message"
	MessageBroadcast        MessageType = "broadcast"
	MessageShutdownRequest  MessageType = "shutdown_request"
	MessageShutdownResponse MessageType = "shutdown_response"
	MessagePlanApproval     MessageType = "plan_approval"
)

// Message is a single communication between team members.
type Message struct {
	ID        string
	Type      MessageType
	From      string
	To        string // empty = route via topology
	Content   string
	RequestID string // for request-response pairing
	Timestamp time.Time
}

// NewMessage creates a message with auto-generated ID and timestamp.
func NewMessage(msgType MessageType, from, to, content string) *Message {
	return &Message{
		ID:        agent.GenerateID("msg"),
		Type:      msgType,
		From:      from,
		To:        to,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// MessageBus routes messages between team members via the team's Topology.
type MessageBus struct {
	subscribers map[string]chan *Message
	topology    Topology
	mu          sync.RWMutex
}

// NewMessageBus creates a bus with the given topology for routing.
func NewMessageBus(topology Topology) *MessageBus {
	return &MessageBus{
		subscribers: make(map[string]chan *Message),
		topology:    topology,
	}
}

// Subscribe registers a member to receive messages.
func (b *MessageBus) Subscribe(name string, bufSize int) <-chan *Message {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan *Message, bufSize)
	b.subscribers[name] = ch
	return ch
}

// Unsubscribe removes a member from the bus.
func (b *MessageBus) Unsubscribe(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subscribers[name]; ok {
		close(ch)
		delete(b.subscribers, name)
	}
}

// Send delivers a message. If msg.To is set, it's a direct message.
// Otherwise, routing is delegated to the topology.
func (b *MessageBus) Send(msg *Message) error {
	if msg.To != "" {
		return b.deliver(msg.To, msg)
	}
	b.mu.RLock()
	members := make([]string, 0, len(b.subscribers))
	for name := range b.subscribers {
		members = append(members, name)
	}
	b.mu.RUnlock()

	targets := b.topology.Route(msg.From, msg, members)
	for _, t := range targets {
		if err := b.deliver(t, msg); err != nil {
			return err
		}
	}
	return nil
}

// Broadcast sends a message to all members except the sender.
func (b *MessageBus) Broadcast(msg *Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for name, ch := range b.subscribers {
		if name == msg.From {
			continue
		}
		select {
		case ch <- msg:
		default:
			// drop if buffer full — log in production
		}
	}
	return nil
}

func (b *MessageBus) deliver(to string, msg *Message) error {
	b.mu.RLock()
	ch, ok := b.subscribers[to]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("member %q not found", to)
	}
	select {
	case ch <- msg:
		return nil
	default:
		return fmt.Errorf("inbox full for member %q", to)
	}
}

// MemberNames returns all subscribed member names.
func (b *MessageBus) MemberNames() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := make([]string, 0, len(b.subscribers))
	for name := range b.subscribers {
		names = append(names, name)
	}
	return names
}
