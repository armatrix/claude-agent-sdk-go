// Package session provides session persistence backends.
package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// MemoryStore is an in-memory session store backed by a sync.RWMutex-protected map.
// Sessions are deep-copied on save and load to prevent external mutation.
type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*agent.Session
}

var _ agent.FullSessionStore = (*MemoryStore)(nil)

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions: make(map[string]*agent.Session),
	}
}

// Save persists a session by deep-copying it into the store.
func (m *MemoryStore) Save(_ context.Context, session *agent.Session) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[session.ID] = deepCopy(session)
	return nil
}

// Load retrieves a session by ID. Returns a deep copy so callers cannot mutate store state.
// Returns an error if the session is not found.
func (m *MemoryStore) Load(_ context.Context, id string) (*agent.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return deepCopy(s), nil
}

// Delete removes a session by ID. Returns an error if not found.
func (m *MemoryStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; !ok {
		return fmt.Errorf("session not found: %s", id)
	}
	delete(m.sessions, id)
	return nil
}

// List returns all sessions in the store as deep copies.
func (m *MemoryStore) List(_ context.Context) ([]*agent.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*agent.Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, deepCopy(s))
	}
	return result, nil
}

// Fork loads a session, clones it with a new ID, saves the clone, and returns it.
func (m *MemoryStore) Fork(_ context.Context, id string) (*agent.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	original, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	forked := original.Clone()
	m.sessions[forked.ID] = deepCopy(forked)
	return forked, nil
}

// deepCopy creates a deep copy of a session.
func deepCopy(s *agent.Session) *agent.Session {
	msgs := make([]anthropic.MessageParam, len(s.Messages))
	copy(msgs, s.Messages)

	return &agent.Session{
		ID:       s.ID,
		Messages: msgs,
		Metadata: agent.SessionMeta{
			Model:       s.Metadata.Model,
			TotalCost:   s.Metadata.TotalCost,
			TotalTokens: s.Metadata.TotalTokens,
			NumTurns:    s.Metadata.NumTurns,
		},
		CreatedAt: s.CreatedAt,
		UpdatedAt: time.Time(s.UpdatedAt),
	}
}
