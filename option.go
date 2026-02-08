package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
)

// AgentOption configures an Agent via the functional options pattern.
type AgentOption func(*agentOptions)

// CompactStrategy selects the compaction implementation.
type CompactStrategy int

const (
	// CompactServer uses the API's server-side compaction (default).
	CompactServer CompactStrategy = iota
	// CompactClient uses SDK-side LLM summarization as a fallback.
	CompactClient
	// CompactDisabled turns off compaction entirely.
	CompactDisabled
)

// CompactConfig controls context compaction behavior.
type CompactConfig struct {
	Strategy          CompactStrategy
	TriggerTokens     int
	PauseAfterCompact bool
	Instructions      string
	FallbackPrompt    string
	PreserveLastN     int
}

// agentOptions holds all configurable fields set via AgentOption functions.
type agentOptions struct {
	model           anthropic.Model
	contextWindow   int
	maxOutputTokens int
	maxTurns        int
	maxBudget       decimal.Decimal
	compact         CompactConfig
	streamBufferSize int

	// Tool configuration (wired in Task 2).
	builtinTools  []string
	disabledTools []string
}

// applyDefaults fills in zero-value fields with sensible defaults.
func (o *agentOptions) applyDefaults() {
	if o.model == "" {
		o.model = DefaultModel
	}
	if o.contextWindow == 0 {
		o.contextWindow = DefaultContextWindow
	}
	if o.maxOutputTokens == 0 {
		o.maxOutputTokens = DefaultMaxOutputTokens
	}
	if o.compact.TriggerTokens == 0 {
		o.compact.TriggerTokens = DefaultCompactTriggerTokens
	}
	if o.compact.PreserveLastN == 0 {
		o.compact.PreserveLastN = 2
	}
	if o.streamBufferSize == 0 {
		o.streamBufferSize = DefaultStreamBufferSize
	}
}

// resolveOptions applies all option functions and fills defaults.
func resolveOptions(opts []AgentOption) agentOptions {
	var o agentOptions
	for _, fn := range opts {
		fn(&o)
	}
	o.applyDefaults()
	return o
}

// --- Model & Context ---

// WithModel sets the Claude model to use.
// Use constants from anthropic-sdk-go, e.g. anthropic.ModelClaudeSonnet4_5.
func WithModel(model anthropic.Model) AgentOption {
	return func(o *agentOptions) { o.model = model }
}

// WithContextWindow sets the context window size in tokens.
func WithContextWindow(tokens int) AgentOption {
	return func(o *agentOptions) { o.contextWindow = tokens }
}

// WithMaxOutputTokens sets the maximum output tokens per response.
func WithMaxOutputTokens(tokens int) AgentOption {
	return func(o *agentOptions) { o.maxOutputTokens = tokens }
}

// WithMaxTurns sets the maximum number of agent loop turns (0 = unlimited).
func WithMaxTurns(n int) AgentOption {
	return func(o *agentOptions) { o.maxTurns = n }
}

// --- Budget ---

// WithBudget sets the maximum budget in USD for a run. Zero means unlimited.
func WithBudget(maxUSD decimal.Decimal) AgentOption {
	return func(o *agentOptions) { o.maxBudget = maxUSD }
}

// --- Compaction ---

// WithCompaction sets the full compaction configuration.
func WithCompaction(config CompactConfig) AgentOption {
	return func(o *agentOptions) { o.compact = config }
}

// WithCompactTrigger is a shortcut to set only the compaction trigger threshold.
func WithCompactTrigger(tokens int) AgentOption {
	return func(o *agentOptions) {
		if tokens < MinCompactTriggerTokens {
			tokens = MinCompactTriggerTokens
		}
		o.compact.TriggerTokens = tokens
	}
}

// WithCompactDisabled disables context compaction entirely.
func WithCompactDisabled() AgentOption {
	return func(o *agentOptions) { o.compact.Strategy = CompactDisabled }
}

// --- Tools ---

// WithBuiltinTools selects which built-in tools to enable by name.
func WithBuiltinTools(names ...string) AgentOption {
	return func(o *agentOptions) { o.builtinTools = names }
}

// WithDisabledTools disables specific tools by name.
func WithDisabledTools(names ...string) AgentOption {
	return func(o *agentOptions) { o.disabledTools = names }
}
