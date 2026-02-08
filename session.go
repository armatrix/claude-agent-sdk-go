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
	Model       anthropic.Model
	TotalCost   decimal.Decimal
	TotalTokens Usage
	NumTurns    int
}

// NewSession creates a new empty session.
func NewSession() *Session {
	now := time.Now()
	return &Session{
		ID:        GenerateID(PrefixSession),
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

