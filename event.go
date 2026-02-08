package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
)

// EventType identifies the kind of event emitted by an AgentStream.
type EventType string

const (
	EventSystem    EventType = "system"
	EventAssistant EventType = "assistant"
	EventUser      EventType = "user"
	EventStream    EventType = "stream"
	EventResult    EventType = "result"
	EventCompact   EventType = "compact"
)

// Event is the interface implemented by all events emitted through AgentStream.
type Event interface {
	Type() EventType
}

// SystemEvent is emitted once at the start of a run with initialization info.
type SystemEvent struct {
	SessionID string
	Model     anthropic.Model
}

func (e *SystemEvent) Type() EventType { return EventSystem }

// AssistantEvent is emitted when the LLM produces a complete response.
type AssistantEvent struct {
	Message anthropic.Message
}

func (e *AssistantEvent) Type() EventType { return EventAssistant }

// StreamEvent is emitted for streaming text deltas as they arrive.
type StreamEvent struct {
	Delta string
}

func (e *StreamEvent) Type() EventType { return EventStream }

// Usage tracks token consumption for a run.
type Usage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
}

// ModelUsage tracks per-model token breakdown.
type ModelUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalCost    decimal.Decimal
}

// ResultEvent is emitted once at the end of a run with summary information.
type ResultEvent struct {
	// Subtype indicates the outcome: "success", "error_max_turns",
	// "error_max_budget_usd", or "error_during_execution".
	Subtype       string
	SessionID     string
	DurationMs    int64
	DurationAPIMs int64
	IsError       bool
	NumTurns      int
	TotalCost     decimal.Decimal
	Usage         Usage
	ModelUsage    map[string]ModelUsage
	Result        string
	Errors        []string
}

func (e *ResultEvent) Type() EventType { return EventResult }

// CompactEvent is emitted when context compaction occurs.
type CompactEvent struct {
	Strategy          CompactStrategy
	TokensBefore      int
	TokensAfter       int
	MessagesRemoved   int
	MessagesRemaining int
}

func (e *CompactEvent) Type() EventType { return EventCompact }
