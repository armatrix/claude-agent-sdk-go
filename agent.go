package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/armatrix/claude-agent-sdk-go/internal/budget"
	"github.com/armatrix/claude-agent-sdk-go/internal/config"
	"github.com/armatrix/claude-agent-sdk-go/internal/engine"
	"github.com/armatrix/claude-agent-sdk-go/internal/hookrunner"
	"github.com/armatrix/claude-agent-sdk-go/permission"
)

// Agent is a stateless execution engine that holds configuration, tools, and hooks.
// The same Agent can be safely shared across multiple goroutines and Clients.
type Agent struct {
	apiClient *anthropic.Client
	tools     *ToolRegistry
	opts      agentOptions

	closeOnce sync.Once
	closeErr  error
}

// NewAgent creates a new Agent with the given options.
// The Agent is stateless — it does not hold any session or conversation history.
func NewAgent(opts ...AgentOption) *Agent {
	// Capture user-set values before applying defaults
	var userSet agentOptions
	for _, fn := range opts {
		fn(&userSet)
	}

	resolved := resolveOptions(opts)

	// Apply settings overrides from JSON config files
	// User-explicit options take precedence over file-based settings
	if len(resolved.settingSources) > 0 {
		settings, err := config.LoadSettings(resolved.settingSources...)
		if err == nil {
			applySettings(&resolved, settings, &userSet)
		}
	}

	// Load skills and prepend to system prompt
	if len(resolved.skillDirs) > 0 {
		skills, err := config.LoadSkills(resolved.skillDirs...)
		if err == nil && len(skills) > 0 {
			skillsPrompt := config.FormatSkillsPrompt(skills)
			resolved.systemPrompt = skillsPrompt + resolved.systemPrompt
		}
	}

	// Resolve system prompt preset (only if no explicit system prompt is set)
	if resolved.systemPrompt == "" && resolved.systemPromptPreset != "" {
		if content, ok := config.GetPreset(string(resolved.systemPromptPreset)); ok {
			resolved.systemPrompt = content
		}
	}

	client := anthropic.NewClient(resolved.clientOptions...)

	a := &Agent{
		apiClient: &client,
		tools:     NewToolRegistry(),
		opts:      resolved,
	}

	// Run post-construction initialization hooks.
	// Sub-packages (subagent, mcp) use these to register tools without import cycles.
	for _, fn := range resolved.onInit {
		fn(a)
	}

	return a
}

// applySettings merges loaded settings into resolved options.
// Options set explicitly via WithXxx take precedence over settings files.
// We check against zero values to detect whether the user set an explicit option.
func applySettings(o *agentOptions, s *config.Settings, userSet *agentOptions) {
	if userSet.model == "" && s.Model != "" {
		o.model = anthropic.Model(s.Model)
	}
	if userSet.systemPrompt == "" && s.SystemPrompt != "" {
		o.systemPrompt = s.SystemPrompt
	}
	if userSet.maxTurns == 0 && s.MaxTurns > 0 {
		o.maxTurns = s.MaxTurns
	}
	if len(userSet.builtinTools) == 0 && len(s.BuiltinTools) > 0 {
		o.builtinTools = s.BuiltinTools
	}
	if len(userSet.disabledTools) == 0 && len(s.DisabledTools) > 0 {
		o.disabledTools = s.DisabledTools
	}
}

// Tools returns the agent's tool registry for registering custom tools.
func (a *Agent) Tools() *ToolRegistry {
	return a.tools
}

// Run starts a single-shot agent execution with a new session.
// Returns an AgentStream for iterating over events.
func (a *Agent) Run(ctx context.Context, prompt string) *AgentStream {
	return a.RunWithSession(ctx, NewSession(), prompt)
}

// RunWithSession starts an agent execution using an existing session.
// The session's message history is preserved and extended.
func (a *Agent) RunWithSession(ctx context.Context, session *Session, prompt string) *AgentStream {
	// Inject workDir, env, and sandbox into context for tool execution
	if a.opts.workDir != "" {
		ctx = WithContextWorkDir(ctx, a.opts.workDir)
	}
	if len(a.opts.env) > 0 {
		ctx = WithContextEnv(ctx, a.opts.env)
	}
	if a.opts.sandbox != nil {
		ctx = WithContextSandbox(ctx, a.opts.sandbox)
	}

	// Build hook runner once (reused for UserPromptSubmit and engine loop)
	var hookRunner *hookrunner.Runner
	if len(a.opts.hookMatchers) > 0 {
		runner, err := hookrunner.New(a.opts.hookMatchers)
		if err == nil {
			hookRunner = runner
		}
	}

	// Fire UserPromptSubmit hook before appending user message
	if hookRunner != nil {
		_, _ = hookRunner.RunUserPromptSubmit(ctx, session.ID, prompt)
	}

	// Append user prompt to session
	session.Messages = append(session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)))

	eventCh := make(chan Event, a.opts.streamBufferSize)
	stream := newStream(eventCh, session)

	// Choose streamer based on compaction strategy and beta flags
	var streamer engine.MessageStreamer
	compactCfg := engine.CompactConfig{
		Strategy:          engine.CompactServer,
		TriggerTokens:     a.opts.compact.TriggerTokens,
		PauseAfterCompact: a.opts.compact.PauseAfterCompact,
		Instructions:      a.opts.compact.Instructions,
	}
	switch {
	case a.opts.compact.Strategy == CompactServer && len(a.opts.betas) > 0:
		streamer = engine.NewCompactStreamerWithBetas(a.apiClient, compactCfg, a.opts.betas)
	case a.opts.compact.Strategy == CompactServer:
		streamer = engine.NewCompactStreamer(a.apiClient, compactCfg)
	case len(a.opts.betas) > 0:
		streamer = engine.NewBetaStreamer(a.apiClient, a.opts.betas)
	default:
		streamer = engine.NewMessageStreamer(&a.apiClient.Messages)
	}

	cfg := engine.LoopConfig{
		Streamer:          streamer,
		Tools:             &toolExecutorAdapter{registry: a.tools},
		Model:             a.opts.model,
		FallbackModel:     a.opts.fallbackModel,
		MaxTokens:         a.opts.maxOutputTokens,
		MaxTurns:          a.opts.maxTurns,
		MaxThinkingTokens: a.opts.maxThinkingTokens,
		Betas:             a.opts.betas,
		Messages:          &session.Messages,
		SessionID:         session.ID,
		Sink:              &channelSink{ch: eventCh},
	}

	// Wire system prompt
	if a.opts.systemPrompt != "" {
		cfg.SystemPrompt = []anthropic.TextBlockParam{
			{Text: a.opts.systemPrompt},
		}
	}

	// Wire budget tracker
	if !a.opts.maxBudget.IsZero() {
		tracker := budget.NewBudgetTracker(a.opts.maxBudget, budget.DefaultPricing)
		cfg.Budget = &budgetAdapter{tracker: tracker}
	}

	// Wire structured output
	if a.opts.outputFormat != nil {
		format := *a.opts.outputFormat
		cfg.OutputToolName = format.Name
		cfg.OutputToolInjector = func(params *anthropic.MessageNewParams) {
			injectOutputTool(params, format)
		}
	}

	// Wire hooks (reuse runner built above)
	if hookRunner != nil {
		cfg.Hooks = &hookRunnerAdapter{runner: hookRunner}
	}

	// Wire permissions
	if a.opts.permissionMode != permission.ModeDefault || a.opts.permissionFunc != nil || len(a.opts.permissionRules) > 0 {
		checker := permission.NewCheckerWithRules(a.opts.permissionMode, a.opts.permissionRules, a.opts.permissionFunc)
		cfg.Permission = &permissionAdapter{checker: checker}
	}

	go func() {
		engine.RunLoop(ctx, cfg)
		close(eventCh)
	}()

	return stream
}

// Model returns the configured model.
func (a *Agent) Model() anthropic.Model {
	return a.opts.model
}

// Options returns a copy of the resolved agent options (for testing/inspection).
func (a *Agent) Options() agentOptions {
	return a.opts
}

// Close releases resources held by the Agent (MCP connections, etc.).
// It is idempotent — calling Close multiple times returns the same result,
// and cleanup callbacks are only executed once.
func (a *Agent) Close() error {
	a.closeOnce.Do(func() {
		var errs []error
		for _, fn := range a.opts.onClose {
			if err := fn(); err != nil {
				errs = append(errs, err)
			}
		}
		a.closeErr = errors.Join(errs...)
	})
	return a.closeErr
}

// AddCleanup registers a cleanup callback that will be called by Close().
//
// This method is NOT goroutine-safe. It must only be called from within
// WithOnInit callbacks during Agent construction (i.e. inside NewAgent).
// Calling it after NewAgent returns may race with Close().
func (a *Agent) AddCleanup(fn func() error) {
	a.opts.onClose = append(a.opts.onClose, fn)
}

// toolExecutorAdapter wraps ToolRegistry to implement internal/agent.ToolExecutor.
type toolExecutorAdapter struct {
	registry *ToolRegistry
}

func (t *toolExecutorAdapter) Execute(ctx context.Context, name string, input json.RawMessage) (string, bool, error) {
	result, err := t.registry.Execute(ctx, name, input)
	if err != nil {
		return "", false, err
	}
	text := extractTextFromBlocks(result.Content)
	return text, result.IsError, nil
}

func (t *toolExecutorAdapter) ListForAPI() []anthropic.ToolUnionParam {
	return t.registry.ListForAPI()
}

// extractTextFromBlocks extracts text from content block param unions.
func extractTextFromBlocks(blocks []anthropic.ContentBlockParamUnion) string {
	for _, b := range blocks {
		if b.OfText != nil {
			return b.OfText.Text
		}
	}
	return ""
}

// channelSink implements internal/agent.EventSink by sending events to a channel.
type channelSink struct {
	ch chan Event
}

func (s *channelSink) OnSystem(sessionID string, model anthropic.Model) {
	s.ch <- &SystemEvent{SessionID: sessionID, Model: model}
}

func (s *channelSink) OnStream(delta string) {
	s.ch <- &StreamEvent{Delta: delta}
}

func (s *channelSink) OnAssistant(msg anthropic.Message) {
	s.ch <- &AssistantEvent{Message: msg}
}

func (s *channelSink) OnCompact(info engine.CompactInfo) {
	strategy := CompactDisabled
	if info.Strategy == engine.CompactServer {
		strategy = CompactServer
	}
	s.ch <- &CompactEvent{Strategy: strategy}
}

func (s *channelSink) OnResult(info engine.ResultInfo) {
	result := extractResultText(info)

	var modelUsage map[string]ModelUsage
	if len(info.ModelUsage) > 0 {
		modelUsage = make(map[string]ModelUsage, len(info.ModelUsage))
		for model, mu := range info.ModelUsage {
			modelUsage[model] = ModelUsage{
				InputTokens:  mu.InputTokens,
				OutputTokens: mu.OutputTokens,
			}
		}
	}

	s.ch <- &ResultEvent{
		Subtype:   info.Subtype,
		SessionID: info.SessionID,
		IsError:   info.IsError,
		NumTurns:  info.NumTurns,
		Usage: Usage{
			InputTokens:              info.InputTokens,
			OutputTokens:             info.OutputTokens,
			CacheReadInputTokens:     info.CacheReadInputTokens,
			CacheCreationInputTokens: info.CacheCreationInputTokens,
		},
		ModelUsage: modelUsage,
		DurationMs: info.DurationMs,
		Result:     result,
		Errors:     info.Errors,
	}
}

func extractResultText(info engine.ResultInfo) string {
	if len(info.Errors) > 0 {
		return fmt.Sprintf("error: %s", info.Errors[0])
	}
	return ""
}

// budgetAdapter wraps budget.BudgetTracker to implement engine.BudgetChecker.
type budgetAdapter struct {
	tracker *budget.BudgetTracker
}

func (b *budgetAdapter) RecordUsage(model anthropic.Model, usage engine.BudgetUsage) {
	b.tracker.RecordUsage(model, budget.Usage{
		InputTokens:              usage.InputTokens,
		OutputTokens:             usage.OutputTokens,
		CacheReadInputTokens:     usage.CacheRead,
		CacheCreationInputTokens: usage.CacheCreation,
	})
}

func (b *budgetAdapter) Exhausted() bool {
	return b.tracker.Exhausted()
}

// hookRunnerAdapter wraps hookrunner.Runner to implement engine.HookRunner.
type hookRunnerAdapter struct {
	runner *hookrunner.Runner
}

func (h *hookRunnerAdapter) RunPreToolUse(ctx context.Context, sessionID, toolName string, input json.RawMessage) (*engine.HookPreToolResult, error) {
	result, err := h.runner.RunPreToolUse(ctx, sessionID, toolName, input)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &engine.HookPreToolResult{
		Block:        result.Block,
		Reason:       result.Reason,
		UpdatedInput: result.UpdatedInput,
	}, nil
}

func (h *hookRunnerAdapter) RunPostToolUse(ctx context.Context, sessionID, toolName string, input json.RawMessage, output string) error {
	return h.runner.RunPostToolUse(ctx, sessionID, toolName, input, output)
}

func (h *hookRunnerAdapter) RunPostToolFailure(ctx context.Context, sessionID, toolName string, input json.RawMessage, toolErr error) error {
	return h.runner.RunPostToolFailure(ctx, sessionID, toolName, input, toolErr)
}

func (h *hookRunnerAdapter) RunStop(ctx context.Context, sessionID string) error {
	return h.runner.RunStop(ctx, sessionID)
}

func (h *hookRunnerAdapter) RunSessionStart(ctx context.Context, sessionID string) error {
	return h.runner.RunSessionStart(ctx, sessionID)
}

func (h *hookRunnerAdapter) RunSessionEnd(ctx context.Context, sessionID string) error {
	return h.runner.RunSessionEnd(ctx, sessionID)
}

func (h *hookRunnerAdapter) RunPreCompact(ctx context.Context, sessionID, strategy string) error {
	return h.runner.RunPreCompact(ctx, sessionID, strategy)
}

func (h *hookRunnerAdapter) RunPostCompact(ctx context.Context, sessionID, strategy string) error {
	return h.runner.RunPostCompact(ctx, sessionID, strategy)
}

func (h *hookRunnerAdapter) RunPreAPIRequest(ctx context.Context, sessionID, model string, messageCount int) error {
	return h.runner.RunPreAPIRequest(ctx, sessionID, model, messageCount)
}

func (h *hookRunnerAdapter) RunPostAPIRequest(ctx context.Context, sessionID, model string, inputTokens, outputTokens int64) error {
	return h.runner.RunPostAPIRequest(ctx, sessionID, model, inputTokens, outputTokens)
}

func (h *hookRunnerAdapter) RunToolResult(ctx context.Context, sessionID, toolName string, input json.RawMessage, output string, isError bool) error {
	return h.runner.RunToolResult(ctx, sessionID, toolName, input, output, isError)
}

func (h *hookRunnerAdapter) RunNotification(ctx context.Context, sessionID, notifType string, payload json.RawMessage) error {
	return h.runner.RunNotification(ctx, sessionID, notifType, payload)
}

func (h *hookRunnerAdapter) RunPermissionRequest(ctx context.Context, sessionID, toolName string, input json.RawMessage) (*engine.HookPreToolResult, error) {
	result, err := h.runner.RunPermissionRequest(ctx, sessionID, toolName, input)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &engine.HookPreToolResult{
		Block:  result.Block,
		Reason: result.Reason,
	}, nil
}

// permissionAdapter wraps permission.Checker to implement engine.PermissionChecker.
type permissionAdapter struct {
	checker *permission.Checker
}

func (p *permissionAdapter) Check(ctx context.Context, toolName string, input json.RawMessage) (int, error) {
	decision, err := p.checker.Check(ctx, toolName, input)
	if err != nil {
		return 0, err
	}
	return int(decision), nil
}
