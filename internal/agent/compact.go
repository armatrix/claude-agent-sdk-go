package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// CompactStrategy selects the compaction implementation.
type CompactStrategy int

const (
	CompactDisabled CompactStrategy = iota
	CompactServer
)

// CompactConfig controls context compaction behavior.
type CompactConfig struct {
	Strategy          CompactStrategy
	TriggerTokens     int
	PauseAfterCompact bool
	Instructions      string
}

// compactAwareStreamer wraps an API client and injects compaction parameters
// when using the Beta API. It converts BetaMessage stream events back to
// standard MessageStreamEventUnion events so the loop stays unchanged.
type compactAwareStreamer struct {
	betaSvc  *anthropic.BetaMessageService
	stdSvc   *anthropic.MessageService
	compact  CompactConfig
}

// NewCompactStreamer creates a MessageStreamer that handles compaction via the Beta API.
// When compact.Strategy == CompactServer, it uses the beta endpoint with context_management.
// Otherwise, it falls back to the standard endpoint.
func NewCompactStreamer(client *anthropic.Client, compact CompactConfig) MessageStreamer {
	return &compactAwareStreamer{
		betaSvc: &client.Beta.Messages,
		stdSvc:  &client.Messages,
		compact: compact,
	}
}

func (s *compactAwareStreamer) NewStreaming(ctx context.Context, params anthropic.MessageNewParams) *ssestream.Stream[anthropic.MessageStreamEventUnion] {
	if s.compact.Strategy != CompactServer {
		return s.stdSvc.NewStreaming(ctx, params)
	}

	// Convert standard params to beta params with context_management
	betaParams := convertToBetaParams(params, s.compact)

	// Call beta API
	betaStream := s.betaSvc.NewStreaming(ctx, betaParams)

	// Wrap the beta stream to convert events to standard format
	return wrapBetaStream(betaStream)
}

// convertToBetaParams converts standard MessageNewParams to BetaMessageNewParams
// with context_management configuration added.
func convertToBetaParams(params anthropic.MessageNewParams, compact CompactConfig) anthropic.BetaMessageNewParams {
	// Convert messages
	betaMessages := make([]anthropic.BetaMessageParam, len(params.Messages))
	for i, msg := range params.Messages {
		betaMessages[i] = convertMessageParam(msg)
	}

	// Convert tools
	betaTools := make([]anthropic.BetaToolUnionParam, len(params.Tools))
	for i, tool := range params.Tools {
		betaTools[i] = convertToolParam(tool)
	}

	// Build context management
	compactEdit := anthropic.BetaCompact20260112EditParam{
		Trigger: anthropic.BetaInputTokensTriggerParam{
			Value: int64(compact.TriggerTokens),
		},
	}

	if compact.PauseAfterCompact {
		compactEdit.PauseAfterCompaction = anthropic.Bool(true)
	}

	if compact.Instructions != "" {
		compactEdit.Instructions = anthropic.String(compact.Instructions)
	}

	betaParams := anthropic.BetaMessageNewParams{
		Model:     params.Model,
		MaxTokens: params.MaxTokens,
		Messages:  betaMessages,
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBeta("compact-2026-01-12")},
		ContextManagement: anthropic.BetaContextManagementConfigParam{
			Edits: []anthropic.BetaContextManagementConfigEditUnionParam{
				{OfCompact20260112: &compactEdit},
			},
		},
	}

	if len(betaTools) > 0 {
		betaParams.Tools = betaTools
	}

	// Convert system prompt if present
	if len(params.System) > 0 {
		betaSystem := make([]anthropic.BetaTextBlockParam, len(params.System))
		for i, s := range params.System {
			betaSystem[i] = anthropic.BetaTextBlockParam{
				Text: s.Text,
			}
		}
		betaParams.System = betaSystem
	}

	return betaParams
}

// convertMessageParam converts a standard MessageParam to a BetaMessageParam.
func convertMessageParam(msg anthropic.MessageParam) anthropic.BetaMessageParam {
	betaContent := make([]anthropic.BetaContentBlockParamUnion, len(msg.Content))
	for i, block := range msg.Content {
		betaContent[i] = convertContentBlockParam(block)
	}
	return anthropic.BetaMessageParam{
		Role:    anthropic.BetaMessageParamRole(msg.Role),
		Content: betaContent,
	}
}

// convertContentBlockParam converts a standard ContentBlockParamUnion to Beta.
func convertContentBlockParam(block anthropic.ContentBlockParamUnion) anthropic.BetaContentBlockParamUnion {
	switch {
	case block.OfText != nil:
		return anthropic.BetaContentBlockParamUnion{
			OfText: &anthropic.BetaTextBlockParam{
				Text: block.OfText.Text,
			},
		}
	case block.OfToolUse != nil:
		return anthropic.BetaContentBlockParamUnion{
			OfToolUse: &anthropic.BetaToolUseBlockParam{
				ID:    block.OfToolUse.ID,
				Name:  block.OfToolUse.Name,
				Input: block.OfToolUse.Input,
			},
		}
	case block.OfToolResult != nil:
		betaContent := make([]anthropic.BetaToolResultBlockParamContentUnion, len(block.OfToolResult.Content))
		for i, c := range block.OfToolResult.Content {
			if c.OfText != nil {
				betaContent[i] = anthropic.BetaToolResultBlockParamContentUnion{
					OfText: &anthropic.BetaTextBlockParam{
						Text: c.OfText.Text,
					},
				}
			}
		}
		return anthropic.BetaContentBlockParamUnion{
			OfToolResult: &anthropic.BetaToolResultBlockParam{
				ToolUseID: block.OfToolResult.ToolUseID,
				Content:   betaContent,
				IsError:   block.OfToolResult.IsError,
			},
		}
	case block.OfThinking != nil:
		return anthropic.BetaContentBlockParamUnion{
			OfThinking: &anthropic.BetaThinkingBlockParam{
				Thinking:  block.OfThinking.Thinking,
				Signature: block.OfThinking.Signature,
			},
		}
	case block.OfRedactedThinking != nil:
		return anthropic.BetaContentBlockParamUnion{
			OfRedactedThinking: &anthropic.BetaRedactedThinkingBlockParam{
				Data: block.OfRedactedThinking.Data,
			},
		}
	default:
		// Fallback: marshal/unmarshal through JSON
		data, _ := json.Marshal(block)
		var beta anthropic.BetaContentBlockParamUnion
		json.Unmarshal(data, &beta)
		return beta
	}
}

// convertToolParam converts a standard ToolUnionParam to Beta.
func convertToolParam(tool anthropic.ToolUnionParam) anthropic.BetaToolUnionParam {
	if tool.OfTool != nil {
		return anthropic.BetaToolUnionParam{
			OfTool: &anthropic.BetaToolParam{
				Name:        tool.OfTool.Name,
				Description: tool.OfTool.Description,
				InputSchema: anthropic.BetaToolInputSchemaParam(tool.OfTool.InputSchema),
			},
		}
	}
	return anthropic.BetaToolUnionParam{}
}

// wrapBetaStream wraps a Beta SSE stream to produce standard MessageStreamEventUnion events.
// Instead of full type conversion, it re-serializes beta events as standard events
// via JSON round-trip, which works because the wire format is structurally compatible.
func wrapBetaStream(betaStream *ssestream.Stream[anthropic.BetaRawMessageStreamEventUnion]) *ssestream.Stream[anthropic.MessageStreamEventUnion] {
	// Create a pipe: the reader side is fed SSE events converted from beta events,
	// the writer side is driven by consuming the beta stream.
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		for betaStream.Next() {
			event := betaStream.Current()

			// Re-serialize the raw JSON event with the same event type
			eventType := event.Type
			rawJSON := event.RawJSON()

			// Handle compaction-specific event types by mapping them
			if eventType == "" {
				continue
			}

			// Write SSE format
			sseData := fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, rawJSON)
			if _, err := pw.Write([]byte(sseData)); err != nil {
				return
			}
		}

		if err := betaStream.Err(); err != nil {
			errJSON := fmt.Sprintf(`{"type":"error","error":{"type":"stream_error","message":"%s"}}`, strings.ReplaceAll(err.Error(), `"`, `\"`))
			fmt.Fprintf(pw, "event: error\ndata: %s\n\n", errJSON)
		}
	}()

	// Create a standard decoder and stream from the pipe
	resp := &http.Response{
		StatusCode: 200,
		Body:       pr,
		Header:     http.Header{},
	}
	decoder := ssestream.NewDecoder(resp)
	return ssestream.NewStream[anthropic.MessageStreamEventUnion](decoder, nil)
}
