package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// MessageStreamer abstracts the Anthropic Messages API so the loop can be tested
// with a mock. Production code passes the real client.Messages.NewStreaming.
type MessageStreamer interface {
	NewStreaming(ctx context.Context, params anthropic.MessageNewParams) *ssestream.Stream[anthropic.MessageStreamEventUnion]
}

// messageServiceAdapter wraps the real anthropic.MessageService to implement MessageStreamer.
type messageServiceAdapter struct {
	svc *anthropic.MessageService
}

func (a *messageServiceAdapter) NewStreaming(ctx context.Context, params anthropic.MessageNewParams) *ssestream.Stream[anthropic.MessageStreamEventUnion] {
	return a.svc.NewStreaming(ctx, params)
}

// NewMessageStreamer wraps a real anthropic.MessageService as a MessageStreamer.
func NewMessageStreamer(svc *anthropic.MessageService) MessageStreamer {
	return &messageServiceAdapter{svc: svc}
}

// ToolExecutor executes a tool by name with raw JSON input.
type ToolExecutor interface {
	Execute(ctx context.Context, name string, input json.RawMessage) (content string, isError bool, err error)
	ListForAPI() []anthropic.ToolUnionParam
}

// EventSink receives events from the loop. The loop calls these methods instead
// of importing root package event types, breaking the import cycle.
type EventSink interface {
	OnSystem(sessionID string, model anthropic.Model)
	OnStream(delta string)
	OnAssistant(msg anthropic.Message)
	OnResult(info ResultInfo)
	OnCompact(info CompactInfo)
}

// BudgetUsage holds token counts for a single API call (used by BudgetChecker).
type BudgetUsage struct {
	InputTokens    int
	OutputTokens   int
	CacheRead      int
	CacheCreation  int
}

// BudgetChecker tracks and enforces budget limits.
// Nil means no budget enforcement.
type BudgetChecker interface {
	RecordUsage(model anthropic.Model, usage BudgetUsage)
	Exhausted() bool
}

// HookPreToolResult is the result of running pre-tool-use hooks.
type HookPreToolResult struct {
	Block        bool
	Reason       string
	UpdatedInput json.RawMessage
}

// HookRunner executes hooks at various points in the agent loop.
// Nil means no hooks.
type HookRunner interface {
	RunPreToolUse(ctx context.Context, sessionID, toolName string, input json.RawMessage) (*HookPreToolResult, error)
	RunPostToolUse(ctx context.Context, sessionID, toolName string, input json.RawMessage, output string) error
	RunPostToolFailure(ctx context.Context, sessionID, toolName string, input json.RawMessage, toolErr error) error
	RunStop(ctx context.Context, sessionID string) error
	RunSessionStart(ctx context.Context, sessionID string) error
	RunSessionEnd(ctx context.Context, sessionID string) error
	RunPreCompact(ctx context.Context, sessionID, strategy string) error
	RunPostCompact(ctx context.Context, sessionID, strategy string) error
	RunPreAPIRequest(ctx context.Context, sessionID, model string, messageCount int) error
	RunPostAPIRequest(ctx context.Context, sessionID, model string, inputTokens, outputTokens int64) error
	RunToolResult(ctx context.Context, sessionID, toolName string, input json.RawMessage, output string, isError bool) error
	RunNotification(ctx context.Context, sessionID, notifType string, payload json.RawMessage) error
	RunPermissionRequest(ctx context.Context, sessionID, toolName string, input json.RawMessage) (*HookPreToolResult, error)
}

// PermissionChecker evaluates whether a tool is allowed to execute.
// Nil means all tools are allowed.
type PermissionChecker interface {
	Check(ctx context.Context, toolName string, input json.RawMessage) (int, error) // 0=allow, 1=deny, 2=ask
}

// CompactInfo contains data for a compaction event.
type CompactInfo struct {
	Strategy CompactStrategy
}

// PerModelUsage tracks token usage for a single model.
type PerModelUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// ResultInfo contains the data for a result event.
type ResultInfo struct {
	Subtype                  string
	SessionID                string
	IsError                  bool
	NumTurns                 int
	DurationMs               int64
	InputTokens              int64
	OutputTokens             int64
	CacheReadInputTokens     int64
	CacheCreationInputTokens int64
	ModelUsage               map[string]PerModelUsage
	Errors                   []string
}

// LoopConfig holds everything the agent loop needs to execute.
type LoopConfig struct {
	Streamer  MessageStreamer
	Tools     ToolExecutor
	Model     anthropic.Model
	MaxTokens int
	MaxTurns  int

	// FallbackModel is used when the primary model returns overloaded/unavailable.
	// Empty means no fallback — errors propagate immediately.
	FallbackModel anthropic.Model

	// MaxThinkingTokens enables extended thinking when > 0.
	// The API Thinking config will be set to enabled with this budget.
	MaxThinkingTokens int64

	// Betas are beta feature flags to include in API requests.
	// These are merged with any internally required betas (e.g. compaction).
	Betas []string

	// Messages is the mutable message history. The loop appends to it.
	Messages *[]anthropic.MessageParam

	// SystemPrompt is prepended to every API call as a system message.
	SystemPrompt []anthropic.TextBlockParam

	SessionID string
	Sink      EventSink

	// Budget tracks token/cost usage and enforces limits. Nil = no limit.
	Budget BudgetChecker

	// Hooks runs user-defined functions at key points. Nil = no hooks.
	Hooks HookRunner

	// Permission checks tool access. Nil = all tools allowed.
	Permission PermissionChecker

	// OutputToolName is the name of the hidden structured output tool.
	// When non-empty, the loop will inject this tool and force tool_choice.
	// The OutputToolInjector callback handles the actual injection.
	OutputToolName string

	// OutputToolInjector modifies API params to inject the structured output tool.
	// Called before each API call when OutputToolName is set.
	OutputToolInjector func(params *anthropic.MessageNewParams)
}

// RunLoop is the core agent execution loop. It runs in the calling goroutine
// and calls Sink methods to emit events. The caller is responsible for
// channel management.
func RunLoop(ctx context.Context, cfg LoopConfig) {
	startTime := time.Now()
	var inputTokens, outputTokens, cacheRead, cacheCreation int64
	modelUsage := make(map[string]PerModelUsage)

	// 1. Emit SystemEvent
	cfg.Sink.OnSystem(cfg.SessionID, cfg.Model)

	// SessionStart hook
	if cfg.Hooks != nil {
		_ = cfg.Hooks.RunSessionStart(ctx, cfg.SessionID)
	}

	// SessionEnd hook — guaranteed to fire on every exit path
	if cfg.Hooks != nil {
		defer func() { _ = cfg.Hooks.RunSessionEnd(ctx, cfg.SessionID) }()
	}

	turns := 0

	for {
		// Check context cancellation
		if ctx.Err() != nil {
			cfg.Sink.OnResult(ResultInfo{
				Subtype:    "error_during_execution",
				SessionID:  cfg.SessionID,
				IsError:    true,
				NumTurns:   turns,
				DurationMs: time.Since(startTime).Milliseconds(),
				ModelUsage: modelUsage,
				Errors:     []string{ctx.Err().Error()},
			})
			return
		}

		// Build API params — use current model (may switch to fallback on retry)
		currentModel := cfg.Model
		params := anthropic.MessageNewParams{
			Model:     currentModel,
			MaxTokens: int64(cfg.MaxTokens),
			Messages:  *cfg.Messages,
		}

		// Enable extended thinking if configured
		if cfg.MaxThinkingTokens > 0 {
			params.Thinking = anthropic.ThinkingConfigParamOfEnabled(cfg.MaxThinkingTokens)
			// Thinking mode requires MaxTokens >= budget + output headroom
			minRequired := cfg.MaxThinkingTokens + 16384
			if params.MaxTokens < minRequired {
				params.MaxTokens = minRequired
			}
		}

		// Set system prompt if configured
		if len(cfg.SystemPrompt) > 0 {
			params.System = cfg.SystemPrompt
		}

		// Add tools if any are registered
		tools := cfg.Tools.ListForAPI()
		if len(tools) > 0 {
			params.Tools = tools
		}

		// Inject structured output tool if configured
		if cfg.OutputToolInjector != nil {
			cfg.OutputToolInjector(&params)
		}

		// PreAPIRequest hook
		if cfg.Hooks != nil {
			_ = cfg.Hooks.RunPreAPIRequest(ctx, cfg.SessionID, string(cfg.Model), len(*cfg.Messages))
		}

		// Call the streaming API
		stream := cfg.Streamer.NewStreaming(ctx, params)
		msg := anthropic.Message{}

		for stream.Next() {
			event := stream.Current()
			if err := msg.Accumulate(event); err != nil {
				cfg.Sink.OnResult(ResultInfo{
					Subtype:              "error_during_execution",
					SessionID:            cfg.SessionID,
					IsError:              true,
					NumTurns:             turns,
					DurationMs:           time.Since(startTime).Milliseconds(),
					InputTokens:          inputTokens,
					OutputTokens:         outputTokens,
					CacheReadInputTokens: cacheRead,
					ModelUsage:           modelUsage,
					Errors:               []string{fmt.Sprintf("accumulate error: %s", err.Error())},
				})
				stream.Close()
				return
			}

			// Emit text deltas for streaming
			if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
				cfg.Sink.OnStream(event.Delta.Text)
			}
		}

		if err := stream.Err(); err != nil {
			stream.Close()

			// Retry with fallback model on overloaded/unavailable errors
			if cfg.FallbackModel != "" && currentModel != cfg.FallbackModel && isRetryableError(err) {
				currentModel = cfg.FallbackModel
				params.Model = currentModel
				msg = anthropic.Message{}

				retryStream := cfg.Streamer.NewStreaming(ctx, params)
				for retryStream.Next() {
					event := retryStream.Current()
					_ = msg.Accumulate(event)
					if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" && event.Delta.Text != "" {
						cfg.Sink.OnStream(event.Delta.Text)
					}
				}
				if retryErr := retryStream.Err(); retryErr != nil {
					retryStream.Close()
					cfg.Sink.OnResult(ResultInfo{
						Subtype:              "error_during_execution",
						SessionID:            cfg.SessionID,
						IsError:              true,
						NumTurns:             turns,
						DurationMs:           time.Since(startTime).Milliseconds(),
						InputTokens:          inputTokens,
						OutputTokens:         outputTokens,
						CacheReadInputTokens: cacheRead,
						ModelUsage:           modelUsage,
						Errors:               []string{fmt.Sprintf("fallback stream error: %s", retryErr.Error())},
					})
					return
				}
				retryStream.Close()
				// Fall through to normal processing with the fallback response
			} else {
				cfg.Sink.OnResult(ResultInfo{
					Subtype:              "error_during_execution",
					SessionID:            cfg.SessionID,
					IsError:              true,
					NumTurns:             turns,
					DurationMs:           time.Since(startTime).Milliseconds(),
					InputTokens:          inputTokens,
					OutputTokens:         outputTokens,
					CacheReadInputTokens: cacheRead,
					ModelUsage:           modelUsage,
					Errors:               []string{fmt.Sprintf("stream error: %s", err.Error())},
				})
				return
			}
		} else {
			stream.Close()
		}

		// Track usage (aggregate + per-model)
		inputTokens += msg.Usage.InputTokens
		outputTokens += msg.Usage.OutputTokens
		cacheRead += msg.Usage.CacheReadInputTokens
		cacheCreation += msg.Usage.CacheCreationInputTokens

		modelKey := string(params.Model)
		mu := modelUsage[modelKey]
		mu.InputTokens += msg.Usage.InputTokens
		mu.OutputTokens += msg.Usage.OutputTokens
		modelUsage[modelKey] = mu

		// PostAPIRequest hook
		if cfg.Hooks != nil {
			_ = cfg.Hooks.RunPostAPIRequest(ctx, cfg.SessionID, string(cfg.Model), msg.Usage.InputTokens, msg.Usage.OutputTokens)
		}

		// Record budget usage if tracker is configured
		if cfg.Budget != nil {
			cfg.Budget.RecordUsage(cfg.Model, BudgetUsage{
				InputTokens:  int(msg.Usage.InputTokens),
				OutputTokens: int(msg.Usage.OutputTokens),
				CacheRead:    int(msg.Usage.CacheReadInputTokens),
				CacheCreation: int(msg.Usage.CacheCreationInputTokens),
			})
			if cfg.Budget.Exhausted() {
				cfg.Sink.OnAssistant(msg)
				*cfg.Messages = append(*cfg.Messages, msg.ToParam())
				cfg.Sink.OnResult(ResultInfo{
					Subtype:                  "error_max_budget_usd",
					SessionID:                cfg.SessionID,
					IsError:                  true,
					NumTurns:                 turns + 1,
					DurationMs:               time.Since(startTime).Milliseconds(),
					InputTokens:              inputTokens,
					OutputTokens:             outputTokens,
					CacheReadInputTokens:     cacheRead,
					CacheCreationInputTokens: cacheCreation,
					ModelUsage:               modelUsage,
					Errors:                   []string{"budget exhausted"},
				})
				return
			}
		}

		// Emit AssistantEvent
		cfg.Sink.OnAssistant(msg)

		// Append assistant message to messages
		*cfg.Messages = append(*cfg.Messages, msg.ToParam())

		// Check stop reason
		switch msg.StopReason {
		case anthropic.StopReasonEndTurn:
			runStopHooks(ctx, cfg)
			cfg.Sink.OnResult(ResultInfo{
				Subtype:                  "success",
				SessionID:                cfg.SessionID,
				NumTurns:                 turns + 1,
				DurationMs:               time.Since(startTime).Milliseconds(),
				InputTokens:              inputTokens,
				OutputTokens:             outputTokens,
				CacheReadInputTokens:     cacheRead,
				CacheCreationInputTokens: cacheCreation,
				ModelUsage:               modelUsage,
			})
			return

		case anthropic.StopReasonMaxTokens:
			runStopHooks(ctx, cfg)
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "error_max_turns",
				SessionID:            cfg.SessionID,
				IsError:              true,
				NumTurns:             turns + 1,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
				ModelUsage:           modelUsage,
				Errors:               []string{"max_tokens reached"},
			})
			return

		case anthropic.StopReasonToolUse:
			// Check if this is a structured output response (hidden tool)
			if cfg.OutputToolName != "" && hasOutputTool(msg.Content, cfg.OutputToolName) {
				runStopHooks(ctx, cfg)
				cfg.Sink.OnResult(ResultInfo{
					Subtype:                  "success",
					SessionID:                cfg.SessionID,
					NumTurns:                 turns + 1,
					DurationMs:               time.Since(startTime).Milliseconds(),
					InputTokens:              inputTokens,
					OutputTokens:             outputTokens,
					CacheReadInputTokens:     cacheRead,
					CacheCreationInputTokens: cacheCreation,
					ModelUsage:               modelUsage,
				})
				return
			}

			// Process tool use blocks (with hooks + permissions)
			toolResults := processToolUse(ctx, cfg, msg.Content)

			// Append tool results as user message
			*cfg.Messages = append(*cfg.Messages,
				anthropic.NewUserMessage(toolResults...))

		case "compaction":
			// Server-side compaction occurred. The API has already modified
			// the message history. Emit compact event and continue the loop.
			if cfg.Hooks != nil {
				_ = cfg.Hooks.RunPreCompact(ctx, cfg.SessionID, "server")
			}
			cfg.Sink.OnCompact(CompactInfo{Strategy: CompactServer})
			if cfg.Hooks != nil {
				_ = cfg.Hooks.RunPostCompact(ctx, cfg.SessionID, "server")
			}
			// Continue the loop — the API will re-send with compacted context

		default:
			// Unknown stop reason, treat as end
			runStopHooks(ctx, cfg)
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "success",
				SessionID:            cfg.SessionID,
				NumTurns:             turns + 1,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
				ModelUsage:           modelUsage,
			})
			return
		}

		turns++

		// Check maxTurns
		if cfg.MaxTurns > 0 && turns >= cfg.MaxTurns {
			runStopHooks(ctx, cfg)
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "error_max_turns",
				SessionID:            cfg.SessionID,
				IsError:              true,
				NumTurns:             turns,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
				ModelUsage:           modelUsage,
				Errors:               []string{"max turns reached"},
			})
			return
		}
	}
}

// runStopHooks runs Stop hooks if a HookRunner is configured.
func runStopHooks(ctx context.Context, cfg LoopConfig) {
	if cfg.Hooks != nil {
		_ = cfg.Hooks.RunStop(ctx, cfg.SessionID)
	}
}

// processToolUse executes each tool_use block with hook and permission integration.
func processToolUse(ctx context.Context, cfg LoopConfig, content []anthropic.ContentBlockUnion) []anthropic.ContentBlockParamUnion {
	var results []anthropic.ContentBlockParamUnion

	for _, block := range content {
		if block.Type != "tool_use" {
			continue
		}

		toolUse := block.AsToolUse()
		toolInput := json.RawMessage(toolUse.Input)

		// 1. Run PreToolUse hooks — may block or modify input
		if cfg.Hooks != nil {
			hookResult, err := cfg.Hooks.RunPreToolUse(ctx, cfg.SessionID, toolUse.Name, toolInput)
			if err != nil {
				results = append(results,
					anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("hook error: %s", err.Error()), true))
				continue
			}
			if hookResult != nil {
				if hookResult.Block {
					reason := hookResult.Reason
					if reason == "" {
						reason = "blocked by hook"
					}
					results = append(results,
						anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("tool blocked: %s", reason), true))
					continue
				}
				if hookResult.UpdatedInput != nil {
					toolInput = hookResult.UpdatedInput
				}
			}
		}

		// 2. Permission check — may deny
		if cfg.Permission != nil {
			decision, err := cfg.Permission.Check(ctx, toolUse.Name, toolInput)
			if err != nil {
				results = append(results,
					anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("permission error: %s", err.Error()), true))
				continue
			}
			if decision == 1 { // Deny
				results = append(results,
					anthropic.NewToolResultBlock(toolUse.ID, "tool execution denied by permission policy", true))
				continue
			}
			if decision == 2 { // Ask — fire PermissionRequest hook for a decision
				if cfg.Hooks != nil {
					hookResult, hookErr := cfg.Hooks.RunPermissionRequest(ctx, cfg.SessionID, toolUse.Name, toolInput)
					if hookErr != nil {
						results = append(results,
							anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("permission hook error: %s", hookErr.Error()), true))
						continue
					}
					if hookResult != nil && hookResult.Block {
						reason := hookResult.Reason
						if reason == "" {
							reason = "blocked by permission hook"
						}
						results = append(results,
							anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("permission denied: %s", reason), true))
						continue
					}
				}
				// No hook or hook allowed — proceed with execution
			}
		}

		// 3. Execute tool
		text, isError, err := cfg.Tools.Execute(ctx, toolUse.Name, toolInput)

		if err != nil {
			// Tool not found or other registry error
			if cfg.Hooks != nil {
				_ = cfg.Hooks.RunPostToolFailure(ctx, cfg.SessionID, toolUse.Name, toolInput, err)
			}
			results = append(results,
				anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("error: %s", err.Error()), true))
			continue
		}

		// 4. Run PostToolUse or PostToolFailure hooks
		if cfg.Hooks != nil {
			if isError {
				_ = cfg.Hooks.RunPostToolFailure(ctx, cfg.SessionID, toolUse.Name, toolInput, fmt.Errorf("%s", text))
			} else {
				_ = cfg.Hooks.RunPostToolUse(ctx, cfg.SessionID, toolUse.Name, toolInput, text)
			}
		}

		results = append(results,
			anthropic.NewToolResultBlock(toolUse.ID, text, isError))

		// 5. Run ToolResult hook (fires for every tool execution regardless of success/failure)
		if cfg.Hooks != nil {
			_ = cfg.Hooks.RunToolResult(ctx, cfg.SessionID, toolUse.Name, toolInput, text, isError)
		}
	}

	return results
}

// hasOutputTool checks if any tool_use block in the content matches the hidden
// structured output tool name.
func hasOutputTool(content []anthropic.ContentBlockUnion, toolName string) bool {
	for _, block := range content {
		if block.Type == "tool_use" && block.AsToolUse().Name == toolName {
			return true
		}
	}
	return false
}

// isRetryableError returns true if the error indicates the model is overloaded
// or unavailable (suitable for fallback retry).
func isRetryableError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "overloaded") ||
		strings.Contains(msg, "model_unavailable") ||
		strings.Contains(msg, "529") ||
		strings.Contains(msg, "503")
}
