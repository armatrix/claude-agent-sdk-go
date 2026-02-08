package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"

	"github.com/armatrix/claude-agent-sdk-go/hook"
	"github.com/armatrix/claude-agent-sdk-go/permission"
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

	// System prompt injected before conversation.
	systemPrompt string

	// Tool configuration.
	builtinTools  []string
	disabledTools []string

	// Session store for persistence.
	sessionStore SessionStore

	// Structured output format. Zero value means no structured output.
	outputFormat *OutputFormat

	// Settings file paths for merged configuration loading.
	settingSources []string

	// Skill directory paths for loading .md skill files.
	skillDirs []string

	// Hook matchers for pre/post tool use callbacks.
	hookMatchers []hook.Matcher

	// Permission mode and optional callback for tool access control.
	permissionMode permission.Mode
	permissionFunc permission.Func
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

// --- System Prompt ---

// WithSystemPrompt sets the system prompt for the agent.
// The system prompt is sent as the first system message in every API call.
func WithSystemPrompt(prompt string) AgentOption {
	return func(o *agentOptions) { o.systemPrompt = prompt }
}

// --- Session ---

// WithSessionStore sets a session store for persistence.
func WithSessionStore(store SessionStore) AgentOption {
	return func(o *agentOptions) { o.sessionStore = store }
}

// --- Structured Output ---

// WithOutputFormat sets a structured output format.
// The agent will inject a hidden tool and force tool_choice to extract structured data.
func WithOutputFormat(format OutputFormat) AgentOption {
	return func(o *agentOptions) { o.outputFormat = &format }
}

// WithOutputFormatType creates and sets a structured output format from a Go struct type.
func WithOutputFormatType[T any](name string) AgentOption {
	format := NewOutputFormatType[T](name)
	return func(o *agentOptions) { o.outputFormat = &format }
}

// --- Accessors (for inspection/testing) ---

// SystemPromptText returns the configured system prompt text.
func (o agentOptions) SystemPromptText() string { return o.systemPrompt }

// MaxTurnsValue returns the configured max turns.
func (o agentOptions) MaxTurnsValue() int { return o.maxTurns }

// OutputFormatName returns the structured output tool name, or empty if not set.
func (o agentOptions) OutputFormatName() string {
	if o.outputFormat != nil {
		return o.outputFormat.Name
	}
	return ""
}

// --- Settings & Skills ---

// WithSettingSources sets JSON file paths for loading merged settings.
// Later paths override earlier ones (user < project < local).
func WithSettingSources(paths ...string) AgentOption {
	return func(o *agentOptions) { o.settingSources = paths }
}

// WithSkillDirs sets directories to scan for .md skill files.
// Skills are prepended to the system prompt.
func WithSkillDirs(dirs ...string) AgentOption {
	return func(o *agentOptions) { o.skillDirs = dirs }
}

// --- Hooks ---

// WithHooks registers hook matchers that fire at various points during execution.
// Hooks can observe, modify, or block tool execution.
func WithHooks(matchers ...hook.Matcher) AgentOption {
	return func(o *agentOptions) { o.hookMatchers = matchers }
}

// --- Permissions ---

// WithPermissionMode sets the permission mode for tool access control.
func WithPermissionMode(mode permission.Mode) AgentOption {
	return func(o *agentOptions) { o.permissionMode = mode }
}

// WithPermissionFunc sets a custom permission callback for tool access control.
// When set, this function is called instead of the mode-based default behavior.
func WithPermissionFunc(fn permission.Func) AgentOption {
	return func(o *agentOptions) { o.permissionFunc = fn }
}
