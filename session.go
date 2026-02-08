package agent

import (
	"context"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
)

// Session holds the conversation state for a single agent run or multi-turn client.
type Session struct {
	ID        string
	Messages  []anthropic.MessageParam
	Metadata  SessionMeta
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SessionMeta contains summary statistics for a session.
type SessionMeta struct {
	Model       string
	TotalCost   decimal.Decimal
	TotalTokens Usage
	NumTurns    int
}

// NewSession creates a new empty session.
func NewSession() *Session {
	now := time.Now()
	return &Session{
		ID:        generateID(),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// SessionStore defines the interface for session persistence backends.
type SessionStore interface {
	Save(ctx context.Context, session *Session) error
	Load(ctx context.Context, id string) (*Session, error)
	Delete(ctx context.Context, id string) error
}

// generateID produces a simple unique identifier for sessions.
// A production implementation would use crypto/rand or UUID.
func generateID() string {
	return time.Now().Format("20060102-150405.000000000")
}
