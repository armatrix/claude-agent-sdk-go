package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock types ---

// mockToolExecutor implements ToolExecutor for testing.
type mockToolExecutor struct {
	tools    map[string]func(ctx context.Context, input json.RawMessage) (string, bool, error)
	apiTools []anthropic.ToolUnionParam
}

func newMockToolExecutor() *mockToolExecutor {
	return &mockToolExecutor{
		tools: make(map[string]func(ctx context.Context, input json.RawMessage) (string, bool, error)),
	}
}

func (m *mockToolExecutor) Register(name string, fn func(ctx context.Context, input json.RawMessage) (string, bool, error)) {
	m.tools[name] = fn
}

func (m *mockToolExecutor) Execute(ctx context.Context, name string, input json.RawMessage) (string, bool, error) {
	fn, ok := m.tools[name]
	if !ok {
		return "", false, fmt.Errorf("tool not found: %s", name)
	}
	return fn(ctx, input)
}

func (m *mockToolExecutor) ListForAPI() []anthropic.ToolUnionParam {
	return m.apiTools
}

// mockStreamer implements MessageStreamer for testing.
// It returns pre-built SSE responses for successive calls.
type mockStreamer struct {
	mu        sync.Mutex
	responses []string // SSE-formatted strings
	callIdx   int
}

func newMockStreamer(responses ...string) *mockStreamer {
	return &mockStreamer{responses: responses}
}

func (m *mockStreamer) NewStreaming(ctx context.Context, params anthropic.MessageNewParams) *ssestream.Stream[anthropic.MessageStreamEventUnion] {
	m.mu.Lock()
	idx := m.callIdx
	m.callIdx++
	m.mu.Unlock()

	if idx >= len(m.responses) {
		// Return an error stream if we run out of responses
		return ssestream.NewStream[anthropic.MessageStreamEventUnion](nil, fmt.Errorf("no more mock responses"))
	}

	body := io.NopCloser(strings.NewReader(m.responses[idx]))
	resp := &http.Response{
		StatusCode: 200,
		Body:       body,
		Header:     http.Header{},
	}
	decoder := ssestream.NewDecoder(resp)
	return ssestream.NewStream[anthropic.MessageStreamEventUnion](decoder, nil)
}

// eventCollector implements EventSink, collecting all events for assertions.
type eventCollector struct {
	mu       sync.Mutex
	systems  []struct{ SessionID, Model string }
	streams  []string
	assists  []anthropic.Message
	results  []ResultInfo
	compacts []CompactInfo
}

func (c *eventCollector) OnSystem(sessionID, model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.systems = append(c.systems, struct{ SessionID, Model string }{sessionID, model})
}

func (c *eventCollector) OnStream(delta string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.streams = append(c.streams, delta)
}

func (c *eventCollector) OnAssistant(msg anthropic.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.assists = append(c.assists, msg)
}

func (c *eventCollector) OnResult(info ResultInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, info)
}

func (c *eventCollector) OnCompact(info CompactInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.compacts = append(c.compacts, info)
}

// --- SSE helpers ---

// buildSSE constructs an SSE-format string from event type/data pairs.
func buildSSE(events ...sseEvent) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, e.Data))
	}
	return sb.String()
}

type sseEvent struct {
	Type string
	Data string
}

// Pre-built SSE events for common patterns.

func messageStart(model string, inputTokens int64) sseEvent {
	return sseEvent{
		Type: "message_start",
		Data: fmt.Sprintf(`{"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"%s","stop_reason":null,"usage":{"input_tokens":%d,"output_tokens":0}}}`, model, inputTokens),
	}
}

func textBlockStart(index int, text string) sseEvent {
	return sseEvent{
		Type: "content_block_start",
		Data: fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"text","text":"%s"}}`, index, text),
	}
}

func textDelta(index int, text string) sseEvent {
	return sseEvent{
		Type: "content_block_delta",
		Data: fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":"%s"}}`, index, text),
	}
}

func blockStop(index int) sseEvent {
	return sseEvent{
		Type: "content_block_stop",
		Data: fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, index),
	}
}

func toolUseStart(index int, id, name string) sseEvent {
	return sseEvent{
		Type: "content_block_start",
		Data: fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":"%s","name":"%s","input":{}}}`, index, id, name),
	}
}

func inputJSONDelta(index int, json string) sseEvent {
	return sseEvent{
		Type: "content_block_delta",
		Data: fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":"%s"}}`, index, json),
	}
}

func messageDelta(stopReason string, outputTokens int64) sseEvent {
	return sseEvent{
		Type: "message_delta",
		Data: fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":"%s","stop_sequence":null},"usage":{"output_tokens":%d}}`, stopReason, outputTokens),
	}
}

func messageStop() sseEvent {
	return sseEvent{
		Type: "message_stop",
		Data: `{"type":"message_stop"}`,
	}
}

// --- Tests ---

func TestRunLoop_SimpleTextResponse(t *testing.T) {
	sse := buildSSE(
		messageStart("claude-opus-4-6", 10),
		textBlockStart(0, ""),
		textDelta(0, "Hello"),
		textDelta(0, " world"),
		blockStop(0),
		messageDelta("end_turn", 5),
		messageStop(),
	)

	streamer := newMockStreamer(sse)
	tools := newMockToolExecutor()
	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hi")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	// Verify system event
	require.Len(t, collector.systems, 1)
	assert.Equal(t, "test-session", collector.systems[0].SessionID)
	assert.Equal(t, "claude-opus-4-6", collector.systems[0].Model)

	// Verify streaming deltas
	assert.Equal(t, []string{"Hello", " world"}, collector.streams)

	// Verify assistant event
	require.Len(t, collector.assists, 1)
	assert.Equal(t, "Hello world", collector.assists[0].Content[0].Text)

	// Verify result
	require.Len(t, collector.results, 1)
	assert.Equal(t, "success", collector.results[0].Subtype)
	assert.False(t, collector.results[0].IsError)
	assert.Equal(t, 1, collector.results[0].NumTurns)

	// Verify messages were appended (user + assistant)
	assert.Len(t, messages, 2)
}

func TestRunLoop_ToolUseFlow(t *testing.T) {
	// First API call: model requests tool use
	sse1 := buildSSE(
		messageStart("claude-opus-4-6", 10),
		toolUseStart(0, "toolu_123", "get_weather"),
		inputJSONDelta(0, `{\"city\": \"SF\"}`),
		blockStop(0),
		messageDelta("tool_use", 20),
		messageStop(),
	)

	// Second API call: model produces text after getting tool result
	sse2 := buildSSE(
		messageStart("claude-opus-4-6", 30),
		textBlockStart(0, ""),
		textDelta(0, "The weather in SF is sunny."),
		blockStop(0),
		messageDelta("end_turn", 10),
		messageStop(),
	)

	streamer := newMockStreamer(sse1, sse2)
	tools := newMockToolExecutor()

	var toolCallCount int
	tools.Register("get_weather", func(ctx context.Context, input json.RawMessage) (string, bool, error) {
		toolCallCount++
		return "72°F, sunny", false, nil
	})

	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in SF?")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	// Tool was called once
	assert.Equal(t, 1, toolCallCount)

	// Two assistant events (tool_use + final text)
	require.Len(t, collector.assists, 2)

	// Final result is success
	require.Len(t, collector.results, 1)
	assert.Equal(t, "success", collector.results[0].Subtype)
	assert.Equal(t, 2, collector.results[0].NumTurns)

	// Messages: user + assistant(tool_use) + user(tool_result) + assistant(text)
	assert.Len(t, messages, 4)
}

func TestRunLoop_ToolError_ContinuesLoop(t *testing.T) {
	// First API call: model requests tool use
	sse1 := buildSSE(
		messageStart("claude-opus-4-6", 10),
		toolUseStart(0, "toolu_456", "failing_tool"),
		inputJSONDelta(0, `{}`),
		blockStop(0),
		messageDelta("tool_use", 10),
		messageStop(),
	)

	// Second API call: model produces text after getting error result
	sse2 := buildSSE(
		messageStart("claude-opus-4-6", 30),
		textBlockStart(0, ""),
		textDelta(0, "The tool failed, sorry."),
		blockStop(0),
		messageDelta("end_turn", 10),
		messageStop(),
	)

	streamer := newMockStreamer(sse1, sse2)
	tools := newMockToolExecutor()

	tools.Register("failing_tool", func(ctx context.Context, input json.RawMessage) (string, bool, error) {
		return "something went wrong", true, nil
	})

	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Use the failing tool")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	// Loop continued after tool error and eventually succeeded
	require.Len(t, collector.results, 1)
	assert.Equal(t, "success", collector.results[0].Subtype)
	assert.Equal(t, 2, collector.results[0].NumTurns)
}

func TestRunLoop_ToolNotFound_ReturnsErrorResult(t *testing.T) {
	// API call with unknown tool
	sse1 := buildSSE(
		messageStart("claude-opus-4-6", 10),
		toolUseStart(0, "toolu_789", "nonexistent_tool"),
		inputJSONDelta(0, `{}`),
		blockStop(0),
		messageDelta("tool_use", 10),
		messageStop(),
	)

	// After error tool result, model ends
	sse2 := buildSSE(
		messageStart("claude-opus-4-6", 30),
		textBlockStart(0, ""),
		textDelta(0, "I couldn't use that tool."),
		blockStop(0),
		messageDelta("end_turn", 10),
		messageStop(),
	)

	streamer := newMockStreamer(sse1, sse2)
	tools := newMockToolExecutor() // no tools registered

	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Use nonexistent tool")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	// The loop handles the error gracefully and continues
	require.Len(t, collector.results, 1)
	assert.Equal(t, "success", collector.results[0].Subtype)
}

func TestRunLoop_MaxTurnsTermination(t *testing.T) {
	// Model keeps requesting tools
	toolUseSSE := buildSSE(
		messageStart("claude-opus-4-6", 10),
		toolUseStart(0, "toolu_loop", "echo"),
		inputJSONDelta(0, `{\"msg\":\"hi\"}`),
		blockStop(0),
		messageDelta("tool_use", 10),
		messageStop(),
	)

	// Provide enough responses for 3 turns
	streamer := newMockStreamer(toolUseSSE, toolUseSSE, toolUseSSE)
	tools := newMockToolExecutor()
	tools.Register("echo", func(ctx context.Context, input json.RawMessage) (string, bool, error) {
		return "echoed", false, nil
	})

	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Loop forever")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		MaxTurns:  2,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	require.Len(t, collector.results, 1)
	assert.Equal(t, "error_max_turns", collector.results[0].Subtype)
	assert.True(t, collector.results[0].IsError)
	assert.Contains(t, collector.results[0].Errors, "max turns reached")
}

func TestRunLoop_MaxTokensStopReason(t *testing.T) {
	sse := buildSSE(
		messageStart("claude-opus-4-6", 10),
		textBlockStart(0, ""),
		textDelta(0, "This is a very long resp"),
		blockStop(0),
		messageDelta("max_tokens", 4096),
		messageStop(),
	)

	streamer := newMockStreamer(sse)
	tools := newMockToolExecutor()
	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Write a novel")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 4096,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	require.Len(t, collector.results, 1)
	assert.Equal(t, "error_max_turns", collector.results[0].Subtype)
	assert.True(t, collector.results[0].IsError)
	assert.Contains(t, collector.results[0].Errors, "max_tokens reached")
}

func TestRunLoop_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	streamer := newMockStreamer() // no responses needed
	tools := newMockToolExecutor()
	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hi")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(ctx, cfg)

	// System event is emitted before cancellation check
	require.Len(t, collector.systems, 1)

	require.Len(t, collector.results, 1)
	assert.Equal(t, "error_during_execution", collector.results[0].Subtype)
	assert.True(t, collector.results[0].IsError)
}

func TestRunLoop_UsageTracking(t *testing.T) {
	// Two-turn conversation to test cumulative usage
	sse1 := buildSSE(
		messageStart("claude-opus-4-6", 100),
		toolUseStart(0, "toolu_u1", "echo"),
		inputJSONDelta(0, `{}`),
		blockStop(0),
		messageDelta("tool_use", 50),
		messageStop(),
	)

	sse2 := buildSSE(
		messageStart("claude-opus-4-6", 200),
		textBlockStart(0, ""),
		textDelta(0, "Done"),
		blockStop(0),
		messageDelta("end_turn", 30),
		messageStop(),
	)

	streamer := newMockStreamer(sse1, sse2)
	tools := newMockToolExecutor()
	tools.Register("echo", func(ctx context.Context, input json.RawMessage) (string, bool, error) {
		return "ok", false, nil
	})

	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Go")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	require.Len(t, collector.results, 1)
	result := collector.results[0]
	assert.Equal(t, "success", result.Subtype)
	// Usage is cumulative: 100+200 input, 50+30 output
	assert.Equal(t, int64(300), result.InputTokens)
	assert.Equal(t, int64(80), result.OutputTokens)
}

func TestRunLoop_StreamError(t *testing.T) {
	// SSE with an error event
	sse := buildSSE(
		messageStart("claude-opus-4-6", 10),
		sseEvent{Type: "error", Data: `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`},
	)

	streamer := newMockStreamer(sse)
	tools := newMockToolExecutor()
	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Hi")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	require.Len(t, collector.results, 1)
	assert.Equal(t, "error_during_execution", collector.results[0].Subtype)
	assert.True(t, collector.results[0].IsError)
}

func TestRunLoop_MultipleToolsInOneResponse(t *testing.T) {
	// Model requests two tools at once
	sse1 := buildSSE(
		messageStart("claude-opus-4-6", 10),
		toolUseStart(0, "toolu_a", "tool_a"),
		inputJSONDelta(0, `{\"key\":\"val_a\"}`),
		blockStop(0),
		toolUseStart(1, "toolu_b", "tool_b"),
		inputJSONDelta(1, `{\"key\":\"val_b\"}`),
		blockStop(1),
		messageDelta("tool_use", 30),
		messageStop(),
	)

	sse2 := buildSSE(
		messageStart("claude-opus-4-6", 50),
		textBlockStart(0, ""),
		textDelta(0, "Both tools done."),
		blockStop(0),
		messageDelta("end_turn", 10),
		messageStop(),
	)

	streamer := newMockStreamer(sse1, sse2)
	tools := newMockToolExecutor()

	var callOrder []string
	tools.Register("tool_a", func(ctx context.Context, input json.RawMessage) (string, bool, error) {
		callOrder = append(callOrder, "a")
		return "result_a", false, nil
	})
	tools.Register("tool_b", func(ctx context.Context, input json.RawMessage) (string, bool, error) {
		callOrder = append(callOrder, "b")
		return "result_b", false, nil
	})

	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Use both tools")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	// Both tools called in order
	assert.Equal(t, []string{"a", "b"}, callOrder)

	require.Len(t, collector.results, 1)
	assert.Equal(t, "success", collector.results[0].Subtype)
}

func TestRunLoop_CompactionStopReason(t *testing.T) {
	// First API call: model triggers compaction
	sse1 := buildSSE(
		messageStart("claude-opus-4-6", 10),
		textBlockStart(0, ""),
		textDelta(0, "compacting..."),
		blockStop(0),
		messageDelta("compaction", 5),
		messageStop(),
	)

	// Second API call: after compaction, model responds normally
	sse2 := buildSSE(
		messageStart("claude-opus-4-6", 10),
		textBlockStart(0, ""),
		textDelta(0, "Done after compaction."),
		blockStop(0),
		messageDelta("end_turn", 10),
		messageStop(),
	)

	streamer := newMockStreamer(sse1, sse2)
	tools := newMockToolExecutor()
	collector := &eventCollector{}

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock("Do something long")),
	}

	cfg := LoopConfig{
		Streamer:  streamer,
		Tools:     tools,
		Model:     "claude-opus-4-6",
		MaxTokens: 1024,
		Messages:  &messages,
		SessionID: "test-session",
		Sink:      collector,
	}

	RunLoop(context.Background(), cfg)

	// Compact event emitted
	require.Len(t, collector.compacts, 1)
	assert.Equal(t, CompactServer, collector.compacts[0].Strategy)

	// Two assistant events (compaction + final)
	require.Len(t, collector.assists, 2)

	// Final result is success
	require.Len(t, collector.results, 1)
	assert.Equal(t, "success", collector.results[0].Subtype)
	assert.Equal(t, 2, collector.results[0].NumTurns)
}
