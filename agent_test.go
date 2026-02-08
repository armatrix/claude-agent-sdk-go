package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/hook"
	"github.com/armatrix/claude-agent-sdk-go/internal/budget"
	"github.com/armatrix/claude-agent-sdk-go/internal/engine"
	"github.com/armatrix/claude-agent-sdk-go/internal/hookrunner"
	"github.com/armatrix/claude-agent-sdk-go/permission"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- extractTextFromBlocks ---

func TestExtractTextFromBlocks_TextBlock(t *testing.T) {
	blocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewTextBlock("hello world"),
	}
	assert.Equal(t, "hello world", extractTextFromBlocks(blocks))
}

func TestExtractTextFromBlocks_Empty(t *testing.T) {
	assert.Equal(t, "", extractTextFromBlocks(nil))
	assert.Equal(t, "", extractTextFromBlocks([]anthropic.ContentBlockParamUnion{}))
}

func TestExtractTextFromBlocks_MultipleBlocks_ReturnsFirst(t *testing.T) {
	blocks := []anthropic.ContentBlockParamUnion{
		anthropic.NewTextBlock("first"),
		anthropic.NewTextBlock("second"),
	}
	assert.Equal(t, "first", extractTextFromBlocks(blocks))
}

// --- toolExecutorAdapter ---

func TestToolExecutorAdapter_Execute(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool(registry, &stubTool{name: "echo", desc: "echo tool"})

	adapter := &toolExecutorAdapter{registry: registry}
	text, isErr, err := adapter.Execute(context.Background(), "echo", json.RawMessage(`{"text":"hi"}`))

	require.NoError(t, err)
	assert.False(t, isErr)
	assert.Equal(t, "echo: hi", text)
}

func TestToolExecutorAdapter_Execute_NotFound(t *testing.T) {
	registry := NewToolRegistry()
	adapter := &toolExecutorAdapter{registry: registry}

	_, _, err := adapter.Execute(context.Background(), "nonexistent", json.RawMessage(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tool not found")
}

func TestToolExecutorAdapter_ListForAPI(t *testing.T) {
	registry := NewToolRegistry()
	RegisterTool(registry, &stubTool{name: "echo", desc: "echo tool"})

	adapter := &toolExecutorAdapter{registry: registry}
	tools := adapter.ListForAPI()

	require.Len(t, tools, 1)
	assert.Equal(t, "echo", tools[0].OfTool.Name)
}

// --- channelSink ---

func TestChannelSink_OnSystem(t *testing.T) {
	ch := make(chan Event, 1)
	sink := &channelSink{ch: ch}

	sink.OnSystem("sess-1", anthropic.ModelClaudeOpus4_6)

	evt := <-ch
	sysEvt, ok := evt.(*SystemEvent)
	require.True(t, ok)
	assert.Equal(t, "sess-1", sysEvt.SessionID)
	assert.Equal(t, anthropic.ModelClaudeOpus4_6, sysEvt.Model)
}

func TestChannelSink_OnStream(t *testing.T) {
	ch := make(chan Event, 1)
	sink := &channelSink{ch: ch}

	sink.OnStream("hello")

	evt := <-ch
	streamEvt, ok := evt.(*StreamEvent)
	require.True(t, ok)
	assert.Equal(t, "hello", streamEvt.Delta)
}

func TestChannelSink_OnAssistant(t *testing.T) {
	ch := make(chan Event, 1)
	sink := &channelSink{ch: ch}

	msg := anthropic.Message{ID: "msg_test"}
	sink.OnAssistant(msg)

	evt := <-ch
	aEvt, ok := evt.(*AssistantEvent)
	require.True(t, ok)
	assert.Equal(t, "msg_test", aEvt.Message.ID)
}

func TestChannelSink_OnCompact(t *testing.T) {
	ch := make(chan Event, 1)
	sink := &channelSink{ch: ch}

	sink.OnCompact(engine.CompactInfo{Strategy: engine.CompactServer})

	evt := <-ch
	cEvt, ok := evt.(*CompactEvent)
	require.True(t, ok)
	assert.Equal(t, CompactServer, cEvt.Strategy)
}

func TestChannelSink_OnResult(t *testing.T) {
	ch := make(chan Event, 1)
	sink := &channelSink{ch: ch}

	sink.OnResult(engine.ResultInfo{
		Subtype:      "success",
		SessionID:    "sess-1",
		NumTurns:     3,
		InputTokens:  100,
		OutputTokens: 50,
	})

	evt := <-ch
	rEvt, ok := evt.(*ResultEvent)
	require.True(t, ok)
	assert.Equal(t, "success", rEvt.Subtype)
	assert.Equal(t, int64(100), rEvt.Usage.InputTokens)
	assert.Equal(t, int64(50), rEvt.Usage.OutputTokens)
	assert.Equal(t, 3, rEvt.NumTurns)
}

func TestChannelSink_OnResult_WithErrors(t *testing.T) {
	ch := make(chan Event, 1)
	sink := &channelSink{ch: ch}

	sink.OnResult(engine.ResultInfo{
		Subtype: "error_during_execution",
		IsError: true,
		Errors:  []string{"stream error: connection reset"},
	})

	evt := <-ch
	rEvt, ok := evt.(*ResultEvent)
	require.True(t, ok)
	assert.True(t, rEvt.IsError)
	assert.Equal(t, "error: stream error: connection reset", rEvt.Result)
}

// --- budgetAdapter ---

func TestBudgetAdapter_RecordUsage(t *testing.T) {
	tracker := budget.NewBudgetTracker(decimal.NewFromFloat(10.0), budget.DefaultPricing)
	adapter := &budgetAdapter{tracker: tracker}

	adapter.RecordUsage(anthropic.ModelClaudeOpus4_6, engine.BudgetUsage{
		InputTokens:  1000,
		OutputTokens: 500,
		CacheRead:    0,
		CacheCreation: 0,
	})

	assert.False(t, adapter.Exhausted())
	assert.True(t, tracker.TotalCost().GreaterThan(decimal.Zero))
}

func TestBudgetAdapter_Exhausted(t *testing.T) {
	// Very tiny budget that will be exhausted immediately
	tracker := budget.NewBudgetTracker(decimal.NewFromFloat(0.000001), budget.DefaultPricing)
	adapter := &budgetAdapter{tracker: tracker}

	adapter.RecordUsage(anthropic.ModelClaudeOpus4_6, engine.BudgetUsage{
		InputTokens:  10000,
		OutputTokens: 10000,
	})

	assert.True(t, adapter.Exhausted())
}

// --- hookRunnerAdapter ---

func TestHookRunnerAdapter_RunPreToolUse_Nil(t *testing.T) {
	matchers := []hook.Matcher{
		{
			Event: hook.PreToolUse,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					return nil, nil // no-op
				},
			},
		},
	}
	runner, err := hookrunner.New(matchers)
	require.NoError(t, err)

	adapter := &hookRunnerAdapter{runner: runner}
	result, err := adapter.RunPreToolUse(context.Background(), "sess", "Read", json.RawMessage(`{}`))

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestHookRunnerAdapter_RunPreToolUse_Block(t *testing.T) {
	matchers := []hook.Matcher{
		{
			Event: hook.PreToolUse,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					return &hook.Result{Block: true, Reason: "not allowed"}, nil
				},
			},
		},
	}
	runner, err := hookrunner.New(matchers)
	require.NoError(t, err)

	adapter := &hookRunnerAdapter{runner: runner}
	result, err := adapter.RunPreToolUse(context.Background(), "sess", "Bash", json.RawMessage(`{}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Block)
	assert.Equal(t, "not allowed", result.Reason)
}

func TestHookRunnerAdapter_RunPreToolUse_UpdateInput(t *testing.T) {
	matchers := []hook.Matcher{
		{
			Event: hook.PreToolUse,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					return &hook.Result{UpdatedInput: json.RawMessage(`{"modified":true}`)}, nil
				},
			},
		},
	}
	runner, err := hookrunner.New(matchers)
	require.NoError(t, err)

	adapter := &hookRunnerAdapter{runner: runner}
	result, err := adapter.RunPreToolUse(context.Background(), "sess", "Write", json.RawMessage(`{}`))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Block)
	assert.JSONEq(t, `{"modified":true}`, string(result.UpdatedInput))
}

func TestHookRunnerAdapter_RunPostToolUse(t *testing.T) {
	var called bool
	matchers := []hook.Matcher{
		{
			Event: hook.PostToolUse,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					called = true
					assert.Equal(t, "Read", input.ToolName)
					assert.Equal(t, "file content", input.ToolOutput)
					return nil, nil
				},
			},
		},
	}
	runner, err := hookrunner.New(matchers)
	require.NoError(t, err)

	adapter := &hookRunnerAdapter{runner: runner}
	err = adapter.RunPostToolUse(context.Background(), "sess", "Read", json.RawMessage(`{}`), "file content")

	require.NoError(t, err)
	assert.True(t, called)
}

func TestHookRunnerAdapter_RunPostToolFailure(t *testing.T) {
	var called bool
	matchers := []hook.Matcher{
		{
			Event: hook.PostToolUseFailure,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					called = true
					assert.Equal(t, "Bash", input.ToolName)
					assert.Error(t, input.ToolError)
					return nil, nil
				},
			},
		},
	}
	runner, err := hookrunner.New(matchers)
	require.NoError(t, err)

	adapter := &hookRunnerAdapter{runner: runner}
	err = adapter.RunPostToolFailure(context.Background(), "sess", "Bash", json.RawMessage(`{}`), assert.AnError)

	require.NoError(t, err)
	assert.True(t, called)
}

func TestHookRunnerAdapter_RunStop(t *testing.T) {
	var called bool
	matchers := []hook.Matcher{
		{
			Event: hook.Stop,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					called = true
					assert.Equal(t, "sess-1", input.SessionID)
					return nil, nil
				},
			},
		},
	}
	runner, err := hookrunner.New(matchers)
	require.NoError(t, err)

	adapter := &hookRunnerAdapter{runner: runner}
	err = adapter.RunStop(context.Background(), "sess-1")

	require.NoError(t, err)
	assert.True(t, called)
}

// --- permissionAdapter ---

func TestPermissionAdapter_Allow(t *testing.T) {
	checker := permission.NewChecker(permission.ModeBypassPermissions, nil)
	adapter := &permissionAdapter{checker: checker}

	decision, err := adapter.Check(context.Background(), "Bash", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, 0, decision) // Allow
}

func TestPermissionAdapter_Deny(t *testing.T) {
	checker := permission.NewChecker(permission.ModePlan, nil)
	adapter := &permissionAdapter{checker: checker}

	decision, err := adapter.Check(context.Background(), "Bash", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, 1, decision) // Deny
}

func TestPermissionAdapter_Ask(t *testing.T) {
	checker := permission.NewChecker(permission.ModeDefault, nil)
	adapter := &permissionAdapter{checker: checker}

	decision, err := adapter.Check(context.Background(), "Bash", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, 2, decision) // Ask
}

func TestPermissionAdapter_ReadOnly_AlwaysAllowed(t *testing.T) {
	checker := permission.NewChecker(permission.ModeDefault, nil)
	adapter := &permissionAdapter{checker: checker}

	for _, tool := range []string{"Read", "Glob", "Grep"} {
		decision, err := adapter.Check(context.Background(), tool, json.RawMessage(`{}`))
		require.NoError(t, err)
		assert.Equal(t, 0, decision, "tool %s should be allowed", tool)
	}
}

func TestPermissionAdapter_CustomFunc(t *testing.T) {
	fn := func(ctx context.Context, toolName string, input json.RawMessage) (permission.Decision, error) {
		if toolName == "dangerous" {
			return permission.Deny, nil
		}
		return permission.Allow, nil
	}
	checker := permission.NewChecker(permission.ModeDefault, fn)
	adapter := &permissionAdapter{checker: checker}

	d1, err := adapter.Check(context.Background(), "safe", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, 0, d1)

	d2, err := adapter.Check(context.Background(), "dangerous", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, 1, d2)
}

// --- RunWithSession wiring ---

func TestRunWithSession_WiresHooks(t *testing.T) {
	a := NewAgent(
		WithMaxTurns(1),
		WithHooks(hook.Matcher{
			Event: hook.Stop,
			Hooks: []hook.Func{
				func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
					return nil, nil
				},
			},
		}),
	)

	// Verify hooks are stored in options
	assert.Len(t, a.opts.hookMatchers, 1)
}

func TestRunWithSession_WiresPermission(t *testing.T) {
	a := NewAgent(
		WithPermissionMode(permission.ModeAcceptEdits),
	)

	assert.Equal(t, permission.ModeAcceptEdits, a.opts.permissionMode)
}

func TestRunWithSession_WiresPermissionFunc(t *testing.T) {
	fn := func(ctx context.Context, toolName string, input json.RawMessage) (permission.Decision, error) {
		return permission.Allow, nil
	}
	a := NewAgent(WithPermissionFunc(fn))

	assert.NotNil(t, a.opts.permissionFunc)
}

func TestRunWithSession_WiresBudget(t *testing.T) {
	budget := decimal.NewFromFloat(5.0)
	a := NewAgent(WithBudget(budget))

	assert.True(t, budget.Equal(a.opts.maxBudget))
}

// --- Option wiring for new fields ---

func TestWithHooks_StoresMatchers(t *testing.T) {
	m1 := hook.Matcher{Event: hook.PreToolUse}
	m2 := hook.Matcher{Event: hook.PostToolUse}
	opts := resolveOptions([]AgentOption{WithHooks(m1, m2)})

	assert.Len(t, opts.hookMatchers, 2)
	assert.Equal(t, hook.PreToolUse, opts.hookMatchers[0].Event)
	assert.Equal(t, hook.PostToolUse, opts.hookMatchers[1].Event)
}

func TestWithPermissionMode_StoresMode(t *testing.T) {
	opts := resolveOptions([]AgentOption{WithPermissionMode(permission.ModePlan)})
	assert.Equal(t, permission.ModePlan, opts.permissionMode)
}

func TestWithPermissionFunc_StoresFunc(t *testing.T) {
	fn := func(ctx context.Context, toolName string, input json.RawMessage) (permission.Decision, error) {
		return permission.Allow, nil
	}
	opts := resolveOptions([]AgentOption{WithPermissionFunc(fn)})
	assert.NotNil(t, opts.permissionFunc)
}

// --- Close and AddCleanup ---

func TestAgent_Close_NoCleanups(t *testing.T) {
	a := NewAgent()
	err := a.Close()
	assert.NoError(t, err)
}

func TestAgent_Close_RunsCleanups(t *testing.T) {
	var calls []string
	a := NewAgent(WithOnInit(func(a *Agent) {
		a.AddCleanup(func() error {
			calls = append(calls, "cleanup1")
			return nil
		})
		a.AddCleanup(func() error {
			calls = append(calls, "cleanup2")
			return nil
		})
	}))

	err := a.Close()
	require.NoError(t, err)
	assert.Equal(t, []string{"cleanup1", "cleanup2"}, calls)
}

func TestAgent_Close_CollectsErrors(t *testing.T) {
	a := NewAgent(WithOnInit(func(a *Agent) {
		a.AddCleanup(func() error { return fmt.Errorf("err1") })
		a.AddCleanup(func() error { return nil }) // success in between
		a.AddCleanup(func() error { return fmt.Errorf("err2") })
	}))

	err := a.Close()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "err1")
	assert.Contains(t, err.Error(), "err2")
}

func TestAgent_Close_Idempotent(t *testing.T) {
	callCount := 0
	a := NewAgent(WithOnInit(func(a *Agent) {
		a.AddCleanup(func() error {
			callCount++
			return nil
		})
	}))

	// First close runs cleanups.
	require.NoError(t, a.Close())
	assert.Equal(t, 1, callCount)

	// Second close is idempotent — cleanups NOT run again.
	require.NoError(t, a.Close())
	assert.Equal(t, 1, callCount)
}

func TestAgent_AddCleanup_FromOnInit(t *testing.T) {
	var cleaned bool
	a := NewAgent(WithOnInit(func(a *Agent) {
		a.AddCleanup(func() error {
			cleaned = true
			return nil
		})
	}))
	require.NoError(t, a.Close())
	assert.True(t, cleaned)
}

// --- Test helpers ---

// stubTool implements Tool[stubInput] for testing.
type stubInput struct {
	Text string `json:"text"`
}

type stubTool struct {
	name string
	desc string
}

func (s *stubTool) Name() string        { return s.name }
func (s *stubTool) Description() string { return s.desc }
func (s *stubTool) Execute(ctx context.Context, input stubInput) (*ToolResult, error) {
	return TextResult("echo: " + input.Text), nil
}

