package teams

import (
	"sync"
	"time"
)

// Blackboard is a shared state space for indirect agent communication.
// Used by the Blackboard topology: agents read/write entries instead of
// sending direct messages.
type Blackboard struct {
	entries map[string]*BoardEntry
	mu      sync.RWMutex
	notify  chan string // entry key change notifications
}

// BoardEntry is a single key-value pair on the blackboard.
type BoardEntry struct {
	Key       string
	Value     any
	Author    string
	UpdatedAt time.Time
}

// NewBlackboard creates an empty blackboard.
func NewBlackboard() *Blackboard {
	return &Blackboard{
		entries: make(map[string]*BoardEntry),
		notify:  make(chan string, 64),
	}
}

// Write sets a key-value pair, notifying subscribers.
func (b *Blackboard) Write(key string, value any, author string) {
	b.mu.Lock()
	b.entries[key] = &BoardEntry{
		Key:       key,
		Value:     value,
		Author:    author,
		UpdatedAt: time.Now(),
	}
	b.mu.Unlock()

	select {
	case b.notify <- key:
	default:
	}
}

// Read returns the entry for the given key, or nil if not found.
func (b *Blackboard) Read(key string) *BoardEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.entries[key]
}

// Keys returns all entry keys.
func (b *Blackboard) Keys() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	keys := make([]string, 0, len(b.entries))
	for k := range b.entries {
		keys = append(keys, k)
	}
	return keys
}

// Notify returns a channel that receives key names when entries change.
func (b *Blackboard) Notify() <-chan string { return b.notify }
