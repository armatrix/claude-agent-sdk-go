package agent

import (
	"context"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
)

// Clone creates a deep copy of the session with a new ID and timestamp.
// The message history is copied so the original session is not affected.
func (s *Session) Clone() *Session {
	msgs := make([]anthropic.MessageParam, len(s.Messages))
	copy(msgs, s.Messages)

	now := time.Now()
	return &Session{
		ID:       generateID(PrefixSession),
		Messages: msgs,
		Metadata: SessionMeta{
			Model:       s.Metadata.Model,
			TotalCost:   s.Metadata.TotalCost,
			TotalTokens: s.Metadata.TotalTokens,
			NumTurns:    s.Metadata.NumTurns,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SessionLister extends SessionStore with the ability to list sessions.
type SessionLister interface {
	SessionStore
	List(ctx context.Context) ([]*Session, error)
}

// SessionForker extends SessionStore with the ability to fork (clone + save) a session.
type SessionForker interface {
	SessionStore
	Fork(ctx context.Context, id string) (*Session, error)
}

// FullSessionStore combines all session store capabilities.
type FullSessionStore interface {
	SessionStore
	List(ctx context.Context) ([]*Session, error)
	Fork(ctx context.Context, id string) (*Session, error)
}
