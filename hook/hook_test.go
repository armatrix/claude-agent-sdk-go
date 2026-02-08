package hook_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/armatrix/claude-agent-sdk-go/hook"
	"github.com/stretchr/testify/assert"
)

func TestEventConstants(t *testing.T) {
	events := []hook.Event{
		hook.PreToolUse,
		hook.PostToolUse,
		hook.PostToolUseFailure,
		hook.Stop,
		hook.SessionStart,
		hook.SessionEnd,
		hook.PreCompact,
		hook.PostCompact,
		hook.PreAPIRequest,
		hook.PostAPIRequest,
		hook.ToolResult,
		hook.Notification,
	}
	seen := make(map[hook.Event]bool, len(events))
	for _, e := range events {
		assert.NotEmpty(t, string(e), "event constant must not be empty")
		assert.False(t, seen[e], "duplicate event constant: %s", e)
		seen[e] = true
	}
}

func TestResultZeroValue(t *testing.T) {
	var r hook.Result
	assert.False(t, r.Block, "zero-value Block should be false")
	assert.Empty(t, r.Reason)
	assert.Nil(t, r.UpdatedInput)
	assert.Empty(t, r.Decision)
}

func TestInputFields(t *testing.T) {
	input := &hook.Input{
		SessionID:  "sess-1",
		Event:      hook.PreToolUse,
		ToolName:   "bash",
		ToolInput:  json.RawMessage(`{"cmd":"ls"}`),
		ToolOutput: "",
		ToolError:  nil,
	}
	assert.Equal(t, "sess-1", input.SessionID)
	assert.Equal(t, hook.PreToolUse, input.Event)
	assert.Equal(t, "bash", input.ToolName)
	assert.JSONEq(t, `{"cmd":"ls"}`, string(input.ToolInput))
}

func TestMatcherDefaults(t *testing.T) {
	m := hook.Matcher{
		Event: hook.PreToolUse,
		Hooks: []hook.Func{
			func(_ context.Context, _ *hook.Input) (*hook.Result, error) {
				return nil, nil
			},
		},
	}
	assert.Equal(t, hook.PreToolUse, m.Event)
	assert.Empty(t, m.Pattern, "empty pattern means match all")
	assert.Len(t, m.Hooks, 1)
	assert.Equal(t, time.Duration(0), m.Timeout, "zero timeout means use default")
}

func TestFuncSignature(t *testing.T) {
	var fn hook.Func = func(ctx context.Context, input *hook.Input) (*hook.Result, error) {
		return &hook.Result{Block: true, Reason: "blocked"}, nil
	}
	res, err := fn(context.Background(), &hook.Input{})
	assert.NoError(t, err)
	assert.True(t, res.Block)
	assert.Equal(t, "blocked", res.Reason)
}
