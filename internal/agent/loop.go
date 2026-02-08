package agent

import (
	"context"
	"encoding/json"
	"fmt"
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
	OnSystem(sessionID, model string)
	OnStream(delta string)
	OnAssistant(msg anthropic.Message)
	OnResult(info ResultInfo)
	OnCompact(info CompactInfo)
}

// CompactInfo contains data for a compaction event.
type CompactInfo struct {
	Strategy CompactStrategy
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
	Errors                   []string
}

// LoopConfig holds everything the agent loop needs to execute.
type LoopConfig struct {
	Streamer  MessageStreamer
	Tools     ToolExecutor
	Model     string
	MaxTokens int
	MaxTurns  int

	// Messages is the mutable message history. The loop appends to it.
	Messages *[]anthropic.MessageParam

	SessionID string
	Sink      EventSink
}

// RunLoop is the core agent execution loop. It runs in the calling goroutine
// and calls Sink methods to emit events. The caller is responsible for
// channel management.
func RunLoop(ctx context.Context, cfg LoopConfig) {
	startTime := time.Now()
	var inputTokens, outputTokens, cacheRead, cacheCreation int64

	// 1. Emit SystemEvent
	cfg.Sink.OnSystem(cfg.SessionID, cfg.Model)

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
				Errors:     []string{ctx.Err().Error()},
			})
			return
		}

		// Build API params
		params := anthropic.MessageNewParams{
			Model:     anthropic.Model(cfg.Model),
			MaxTokens: int64(cfg.MaxTokens),
			Messages:  *cfg.Messages,
		}

		// Add tools if any are registered
		tools := cfg.Tools.ListForAPI()
		if len(tools) > 0 {
			params.Tools = tools
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
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "error_during_execution",
				SessionID:            cfg.SessionID,
				IsError:              true,
				NumTurns:             turns,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
				Errors:               []string{fmt.Sprintf("stream error: %s", err.Error())},
			})
			return
		}
		stream.Close()

		// Track usage
		inputTokens += msg.Usage.InputTokens
		outputTokens += msg.Usage.OutputTokens
		cacheRead += msg.Usage.CacheReadInputTokens
		cacheCreation += msg.Usage.CacheCreationInputTokens

		// Emit AssistantEvent
		cfg.Sink.OnAssistant(msg)

		// Append assistant message to messages
		*cfg.Messages = append(*cfg.Messages, msg.ToParam())

		// Check stop reason
		switch msg.StopReason {
		case anthropic.StopReasonEndTurn:
			cfg.Sink.OnResult(ResultInfo{
				Subtype:                  "success",
				SessionID:                cfg.SessionID,
				NumTurns:                 turns + 1,
				DurationMs:               time.Since(startTime).Milliseconds(),
				InputTokens:              inputTokens,
				OutputTokens:             outputTokens,
				CacheReadInputTokens:     cacheRead,
				CacheCreationInputTokens: cacheCreation,
			})
			return

		case anthropic.StopReasonMaxTokens:
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "error_max_turns",
				SessionID:            cfg.SessionID,
				IsError:              true,
				NumTurns:             turns + 1,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
				Errors:               []string{"max_tokens reached"},
			})
			return

		case anthropic.StopReasonToolUse:
			// Process tool use blocks
			toolResults := processToolUse(ctx, cfg.Tools, msg.Content)

			// Append tool results as user message
			*cfg.Messages = append(*cfg.Messages,
				anthropic.NewUserMessage(toolResults...))

		case "compaction":
			// Server-side compaction occurred. The API has already modified
			// the message history. Emit compact event and continue the loop.
			cfg.Sink.OnCompact(CompactInfo{Strategy: CompactServer})
			// Continue the loop — the API will re-send with compacted context

		default:
			// Unknown stop reason, treat as end
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "success",
				SessionID:            cfg.SessionID,
				NumTurns:             turns + 1,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
			})
			return
		}

		turns++

		// Check maxTurns
		if cfg.MaxTurns > 0 && turns >= cfg.MaxTurns {
			cfg.Sink.OnResult(ResultInfo{
				Subtype:              "error_max_turns",
				SessionID:            cfg.SessionID,
				IsError:              true,
				NumTurns:             turns,
				DurationMs:           time.Since(startTime).Milliseconds(),
				InputTokens:          inputTokens,
				OutputTokens:         outputTokens,
				CacheReadInputTokens: cacheRead,
				Errors:               []string{"max turns reached"},
			})
			return
		}
	}
}

// processToolUse executes each tool_use block and returns tool_result content blocks.
func processToolUse(ctx context.Context, executor ToolExecutor, content []anthropic.ContentBlockUnion) []anthropic.ContentBlockParamUnion {
	var results []anthropic.ContentBlockParamUnion

	for _, block := range content {
		if block.Type != "tool_use" {
			continue
		}

		toolUse := block.AsToolUse()
		text, isError, err := executor.Execute(ctx, toolUse.Name, json.RawMessage(toolUse.Input))

		if err != nil {
			// Tool not found or other registry error
			results = append(results,
				anthropic.NewToolResultBlock(toolUse.ID, fmt.Sprintf("error: %s", err.Error()), true))
			continue
		}

		results = append(results,
			anthropic.NewToolResultBlock(toolUse.ID, text, isError))
	}

	return results
}
