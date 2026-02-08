package agent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

// SystemPromptPreset identifies a built-in system prompt template.
type SystemPromptPreset string

const (
	// PresetDefault means no preset — use the custom system prompt as-is.
	PresetDefault SystemPromptPreset = ""
	// PresetClaudeCode selects the Claude Code system prompt.
	PresetClaudeCode SystemPromptPreset = "claude_code"
)

// agentOptions holds all configurable fields set via AgentOption functions.
type agentOptions struct {
	model             anthropic.Model
	contextWindow     int
	maxOutputTokens   int
	maxTurns          int
	maxThinkingTokens int64
	maxBudget         decimal.Decimal
	compact           CompactConfig
	streamBufferSize  int
	betas             []string

	// System prompt injected before conversation.
	systemPrompt       string
	systemPromptPreset SystemPromptPreset

	// Tool configuration.
	builtinTools        []string
	disabledTools       []string
	toolSearch          bool
	toolSearchThreshold float64

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

	// Permission mode, rules, and optional callback for tool access control.
	permissionMode  permission.Mode
	permissionRules []permission.Rule
	permissionFunc  permission.Func

	// Working directory for tool execution (Bash cmd.Dir, file path resolution).
	workDir string

	// Environment variables merged into tool execution context.
	env map[string]string

	// Anthropic client options (auth provider, base URL, etc.).
	// Passed directly to anthropic.NewClient().
	clientOptions []option.RequestOption

	// Post-construction initialization callbacks. Sub-packages (subagent, mcp)
	// use these to inject setup logic without import cycles.
	onInit []func(*Agent)

	// Fallback model used when the primary model returns overloaded/unavailable.
	fallbackModel anthropic.Model

	// Sandbox configuration for restricting tool execution.
	sandbox *SandboxConfig

	// Slash command directories to scan for .md command files.
	commandDirs []string

	// Cleanup callbacks executed by Agent.Close().
	onClose []func() error
}

// SandboxConfig controls tool execution restrictions.
type SandboxConfig struct {
	// AllowedDirs restricts file tools (Read/Write/Edit/Glob/Grep) to these directories.
	// Empty means no restriction.
	AllowedDirs []string

	// BlockedCommands prevents Bash from executing these commands.
	BlockedCommands []string

	// AllowNetwork controls whether Bash commands can access the network.
	// Default true (allowed).
	AllowNetwork bool
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

// WithMaxThinkingTokens enables extended thinking with the given budget.
// When set to a positive value, the API will use thinking mode with
// BudgetTokens set to this value. Zero (default) disables thinking.
func WithMaxThinkingTokens(n int64) AgentOption {
	return func(o *agentOptions) { o.maxThinkingTokens = n }
}

// WithBetas sets the beta feature flags to include in API requests.
// These are merged with any internally required betas (e.g. compaction).
func WithBetas(betas ...string) AgentOption {
	return func(o *agentOptions) { o.betas = betas }
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

// WithToolSearch enables the ToolSearch meta-tool. When enabled, if the tool
// schema token estimate exceeds the threshold ratio of the context window,
// the full tool list is replaced with just the ToolSearch tool for that API call.
func WithToolSearch(enabled bool) AgentOption {
	return func(o *agentOptions) { o.toolSearch = enabled }
}

// WithToolSearchThreshold sets the ratio of tool schema tokens to context window
// that triggers ToolSearch mode. Default is 0.1 (10%).
func WithToolSearchThreshold(ratio float64) AgentOption {
	return func(o *agentOptions) { o.toolSearchThreshold = ratio }
}

// --- System Prompt ---

// WithSystemPrompt sets the system prompt for the agent.
// The system prompt is sent as the first system message in every API call.
func WithSystemPrompt(prompt string) AgentOption {
	return func(o *agentOptions) { o.systemPrompt = prompt }
}

// WithSystemPromptPreset selects a built-in system prompt preset.
// If a custom system prompt is also set via WithSystemPrompt, the custom
// prompt takes precedence and the preset is ignored.
func WithSystemPromptPreset(preset SystemPromptPreset) AgentOption {
	return func(o *agentOptions) { o.systemPromptPreset = preset }
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

// WithPermissionRules sets declarative permission rules with glob pattern matching.
// Rules are evaluated before the mode-based defaults. Deny rules take priority
// over Ask rules, which take priority over Allow rules.
func WithPermissionRules(rules ...permission.Rule) AgentOption {
	return func(o *agentOptions) { o.permissionRules = append(o.permissionRules, rules...) }
}

// WithAllowedTools is a convenience that creates Allow rules for the given patterns.
// Patterns support glob matching (e.g. "mcp__*", "Read", "Edit").
func WithAllowedTools(patterns ...string) AgentOption {
	return func(o *agentOptions) {
		for _, p := range patterns {
			o.permissionRules = append(o.permissionRules, permission.Rule{
				Pattern:  p,
				Decision: permission.Allow,
			})
		}
	}
}

// WithDisallowedTools is a convenience that creates Deny rules for the given patterns.
// Patterns support glob matching (e.g. "Bash", "mcp__*").
func WithDisallowedTools(patterns ...string) AgentOption {
	return func(o *agentOptions) {
		for _, p := range patterns {
			o.permissionRules = append(o.permissionRules, permission.Rule{
				Pattern:  p,
				Decision: permission.Deny,
			})
		}
	}
}

// --- Working Directory & Environment ---

// WithWorkDir sets the working directory for tool execution.
// Bash commands will use this as cmd.Dir, and file tools will resolve
// relative paths against it.
func WithWorkDir(dir string) AgentOption {
	return func(o *agentOptions) { o.workDir = dir }
}

// WithEnv sets environment variables that are merged into tool execution.
// These are added on top of the inherited OS environment.
func WithEnv(env map[string]string) AgentOption {
	return func(o *agentOptions) { o.env = env }
}

// --- Client Options (Authentication) ---

// WithClientOptions sets raw anthropic-sdk-go request options.
// Use this for custom authentication (Bedrock, Vertex, API key), base URL overrides, etc.
//
// Example:
//
//	// AWS Bedrock
//	agent.NewAgent(agent.WithClientOptions(bedrock.WithLoadDefaultConfig(ctx)))
//
//	// Google Vertex AI
//	agent.NewAgent(agent.WithClientOptions(vertex.WithGoogleAuth(ctx, region, project)))
//
//	// Custom API key
//	agent.NewAgent(agent.WithClientOptions(option.WithAPIKey("sk-...")))
func WithClientOptions(opts ...option.RequestOption) AgentOption {
	return func(o *agentOptions) { o.clientOptions = opts }
}

// --- Fallback Model ---

// WithFallbackModel sets a fallback model to use when the primary model returns
// an overloaded or model_unavailable error. The loop will retry once with this model.
func WithFallbackModel(model anthropic.Model) AgentOption {
	return func(o *agentOptions) { o.fallbackModel = model }
}

// --- Sandbox ---

// WithSandbox sets sandbox restrictions for tool execution.
// AllowedDirs restricts file operations, BlockedCommands prevents specific commands.
func WithSandbox(config SandboxConfig) AgentOption {
	return func(o *agentOptions) { o.sandbox = &config }
}

// --- Slash Commands ---

// WithCommandDirs sets directories to scan for .md slash command files.
// Commands are loaded from files like /commit.md, /review.md, etc.
func WithCommandDirs(dirs ...string) AgentOption {
	return func(o *agentOptions) { o.commandDirs = dirs }
}

// --- Initialization Hooks ---

// WithOnInit registers a callback that runs after Agent construction.
// Sub-packages use this to inject setup logic without creating import cycles.
// Callbacks are executed in registration order.
func WithOnInit(fn func(*Agent)) AgentOption {
	return func(o *agentOptions) {
		o.onInit = append(o.onInit, fn)
	}
}
