package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- NewClient ---

func TestNewClient_CreatesSessionAndAgent(t *testing.T) {
	c := NewClient(
		WithModel(anthropic.ModelClaudeSonnet4_5),
		WithMaxTurns(5),
	)

	require.NotNil(t, c.agent)
	require.NotNil(t, c.session)
	assert.NotEmpty(t, c.session.ID)
	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, c.agent.Model())
}

func TestNewClient_WithSessionStore(t *testing.T) {
	store := &recordingStore{}
	c := NewClient(WithSessionStore(store))

	assert.NotNil(t, c.store)
}

func TestNewClient_NoStore(t *testing.T) {
	c := NewClient()
	assert.Nil(t, c.store)
}

// --- Session / Agent accessors ---

func TestClient_Session(t *testing.T) {
	c := NewClient()
	sess := c.Session()

	require.NotNil(t, sess)
	assert.Equal(t, c.session, sess)
}

func TestClient_Agent(t *testing.T) {
	c := NewClient()
	a := c.Agent()

	require.NotNil(t, a)
	assert.Equal(t, c.agent, a)
}

// --- SetModel ---

func TestClient_SetModel(t *testing.T) {
	c := NewClient(WithModel(anthropic.ModelClaudeOpus4_6))
	assert.Equal(t, anthropic.ModelClaudeOpus4_6, c.Agent().Model())

	c.SetModel(anthropic.ModelClaudeSonnet4_5)
	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, c.Agent().Model())
}

// --- Fork ---

func TestClient_Fork_IndependentSession(t *testing.T) {
	c := NewClient()
	c.session.Messages = append(c.session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("hello")))

	// Need unique IDs — small sleep for timestamp-based ID generation
	time.Sleep(time.Millisecond)
	forked := c.Fork()

	// Different session IDs
	assert.NotEqual(t, c.session.ID, forked.session.ID)

	// Same message count initially
	assert.Len(t, forked.session.Messages, 1)

	// Modifying forked session doesn't affect original
	forked.session.Messages = append(forked.session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("extra")))
	assert.Len(t, c.session.Messages, 1)
	assert.Len(t, forked.session.Messages, 2)
}

func TestClient_Fork_SharesAgent(t *testing.T) {
	c := NewClient()
	forked := c.Fork()

	// Both clients share the same Agent pointer
	assert.Same(t, c.agent, forked.agent)
}

func TestClient_Fork_SharesStore(t *testing.T) {
	store := &recordingStore{}
	c := NewClient(WithSessionStore(store))
	forked := c.Fork()

	assert.Same(t, c.store, forked.store)
}

// --- Resume ---

func TestClient_Resume_LoadsSession(t *testing.T) {
	store := &recordingStore{sessions: make(map[string]*Session)}
	savedSession := NewSession()
	savedSession.ID = "saved-id"
	savedSession.Messages = append(savedSession.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("saved message")))
	store.sessions["saved-id"] = savedSession

	c := NewClient(WithSessionStore(store))
	originalID := c.session.ID

	err := c.Resume(context.Background(), "saved-id")
	require.NoError(t, err)

	// Session was replaced
	assert.NotEqual(t, originalID, c.session.ID)
	assert.Equal(t, "saved-id", c.session.ID)
	assert.Len(t, c.session.Messages, 1)
}

func TestClient_Resume_NoStore(t *testing.T) {
	c := NewClient() // no store
	err := c.Resume(context.Background(), "any-id")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrNoSessionStore)
}

func TestClient_Resume_NotFound(t *testing.T) {
	store := &recordingStore{}
	c := NewClient(WithSessionStore(store))

	err := c.Resume(context.Background(), "nonexistent")
	assert.Error(t, err)
}

// --- Close ---

func TestClient_Close_SavesSession(t *testing.T) {
	store := &recordingStore{}
	c := NewClient(WithSessionStore(store))

	err := c.Close()
	require.NoError(t, err)

	assert.Equal(t, 1, store.saveCount)
	assert.Contains(t, store.sessions, c.session.ID)
}

func TestClient_Close_NoStore(t *testing.T) {
	c := NewClient()
	err := c.Close()
	assert.NoError(t, err) // should be no-op, not panic
}

func TestClient_Close_StoreError(t *testing.T) {
	store := &recordingStore{saveErr: fmt.Errorf("disk full")}
	c := NewClient(WithSessionStore(store))

	err := c.Close()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

// --- Interrupt ---

func TestClient_Interrupt_NilCancel(t *testing.T) {
	c := NewClient()
	// Should not panic when no Query is running
	c.Interrupt()
}

func TestClient_Interrupt_CancelsContext(t *testing.T) {
	c := NewClient()

	// Simulate Query setting cancel
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancel = cancel
	c.mu.Unlock()

	c.Interrupt()

	// Context should be cancelled
	assert.Error(t, ctx.Err())
}

// --- Concurrent safety ---

func TestClient_ConcurrentSetModel(t *testing.T) {
	c := NewClient()
	var wg sync.WaitGroup

	models := []anthropic.Model{
		anthropic.ModelClaudeOpus4_6,
		anthropic.ModelClaudeSonnet4_5,
		anthropic.ModelClaudeHaiku4_5,
	}

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c.SetModel(models[idx%len(models)])
		}(i)
	}

	wg.Wait()

	// Should not panic and model should be one of the valid models
	model := c.Agent().Model()
	assert.Contains(t, models, model)
}

func TestClient_ConcurrentInterrupt(t *testing.T) {
	c := NewClient()
	var wg sync.WaitGroup

	// Multiple interrupts should not panic
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Interrupt()
		}()
	}

	wg.Wait()
}

// --- recordingStore is a test SessionStore that records calls ---

type recordingStore struct {
	mu        sync.Mutex
	sessions  map[string]*Session
	saveCount int
	saveErr   error
}

func init() {
	// Ensure recordingStore initializes its map
}

func (s *recordingStore) Save(_ context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.sessions == nil {
		s.sessions = make(map[string]*Session)
	}
	s.sessions[session.ID] = session
	s.saveCount++
	return nil
}

func (s *recordingStore) Load(_ context.Context, id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}
	return sess, nil
}

func (s *recordingStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions != nil {
		delete(s.sessions, id)
	}
	return nil
}
