package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"

	internalagent "github.com/armatrix/claude-agent-sdk-go/internal/agent"
)

// Agent is a stateless execution engine that holds configuration, tools, and hooks.
// The same Agent can be safely shared across multiple goroutines and Clients.
type Agent struct {
	apiClient *anthropic.Client
	tools     *ToolRegistry
	opts      agentOptions
}

// NewAgent creates a new Agent with the given options.
// The Agent is stateless — it does not hold any session or conversation history.
func NewAgent(opts ...AgentOption) *Agent {
	resolved := resolveOptions(opts)

	client := anthropic.NewClient()

	return &Agent{
		apiClient: &client,
		tools:     NewToolRegistry(),
		opts:      resolved,
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
	// Append user prompt to session
	session.Messages = append(session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)))

	eventCh := make(chan Event, a.opts.streamBufferSize)
	stream := newStream(eventCh, session)

	// Choose streamer based on compaction strategy
	var streamer internalagent.MessageStreamer
	if a.opts.compact.Strategy == CompactServer {
		streamer = internalagent.NewCompactStreamer(a.apiClient, internalagent.CompactConfig{
			Strategy:          internalagent.CompactServer,
			TriggerTokens:     a.opts.compact.TriggerTokens,
			PauseAfterCompact: a.opts.compact.PauseAfterCompact,
			Instructions:      a.opts.compact.Instructions,
		})
	} else {
		streamer = internalagent.NewMessageStreamer(&a.apiClient.Messages)
	}

	cfg := internalagent.LoopConfig{
		Streamer:  streamer,
		Tools:     &toolExecutorAdapter{registry: a.tools},
		Model:     a.opts.model,
		MaxTokens: a.opts.maxOutputTokens,
		MaxTurns:  a.opts.maxTurns,
		Messages:  &session.Messages,
		SessionID: session.ID,
		Sink:      &channelSink{ch: eventCh},
	}

	go func() {
		internalagent.RunLoop(ctx, cfg)
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

func (s *channelSink) OnCompact(info internalagent.CompactInfo) {
	strategy := CompactDisabled
	if info.Strategy == internalagent.CompactServer {
		strategy = CompactServer
	}
	s.ch <- &CompactEvent{Strategy: strategy}
}

func (s *channelSink) OnResult(info internalagent.ResultInfo) {
	result := extractResultText(info)
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
		DurationMs: info.DurationMs,
		Result:     result,
		Errors:     info.Errors,
	}
}

func extractResultText(info internalagent.ResultInfo) string {
	if len(info.Errors) > 0 {
		return fmt.Sprintf("error: %s", info.Errors[0])
	}
	return ""
}
