package agent

// Model and context window defaults.
const (
	// DefaultModel is the default Claude model used when no model is specified.
	DefaultModel = "claude-opus-4-6"

	// DefaultContextWindow is the standard 200K context window.
	DefaultContextWindow = 200_000

	// ContextWindow1M enables the 1M context window beta (requires context-1m-2025-08-07 header).
	ContextWindow1M = 1_000_000

	// DefaultMaxOutputTokens is the default maximum output tokens per response.
	DefaultMaxOutputTokens = 16_384

	// MaxOutputTokensOpus46 is the maximum output tokens for Claude Opus 4.6.
	MaxOutputTokensOpus46 = 128_000

	// DefaultCompactTriggerTokens is the default token count that triggers compaction.
	DefaultCompactTriggerTokens = 150_000

	// MinCompactTriggerTokens is the minimum allowed compaction trigger threshold.
	MinCompactTriggerTokens = 50_000

	// DefaultMaxTurns is the default max turns (0 = unlimited).
	DefaultMaxTurns = 0

	// DefaultStreamBufferSize is the default channel buffer size for streaming events.
	DefaultStreamBufferSize = 64
)
