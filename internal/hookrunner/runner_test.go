package hookrunner_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	pubhook "github.com/armatrix/claude-agent-sdk-go/hook"
	"github.com/armatrix/claude-agent-sdk-go/internal/hookrunner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noop(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
	return nil, nil
}

func blockHook(reason string) pubhook.Func {
	return func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
		return &pubhook.Result{Block: true, Reason: reason}, nil
	}
}

func allowHook() pubhook.Func {
	return func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
		return &pubhook.Result{}, nil
	}
}

func TestNewInvalidPattern(t *testing.T) {
	_, err := hookrunner.New([]pubhook.Matcher{
		{Event: pubhook.PreToolUse, Pattern: "[invalid", Hooks: []pubhook.Func{noop}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestEmptyRunnerReturnsNil(t *testing.T) {
	r, err := hookrunner.New(nil)
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "sess", "bash", nil)
	require.NoError(t, err)
	assert.Nil(t, res, "empty runner should return nil result")
}

func TestBasicMatchByEvent(t *testing.T) {
	called := false
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					called = true
					assert.Equal(t, "sess-1", in.SessionID)
					assert.Equal(t, pubhook.PreToolUse, in.Event)
					assert.Equal(t, "bash", in.ToolName)
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "sess-1", "bash", nil)
	require.NoError(t, err)
	assert.True(t, called)
	assert.Nil(t, res)
}

func TestEventMismatchSkips(t *testing.T) {
	called := false
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.Stop,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					called = true
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "sess", "bash", nil)
	require.NoError(t, err)
	assert.False(t, called, "Stop matcher should not fire for PreToolUse")
	assert.Nil(t, res)
}

func TestRegexPatternMatching(t *testing.T) {
	var matched []string
	hook := func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
		matched = append(matched, in.ToolName)
		return nil, nil
	}

	r, err := hookrunner.New([]pubhook.Matcher{
		{Event: pubhook.PreToolUse, Pattern: `^bash$`, Hooks: []pubhook.Func{hook}},
		{Event: pubhook.PreToolUse, Pattern: `^file_`, Hooks: []pubhook.Func{hook}},
	})
	require.NoError(t, err)

	// "bash" matches first matcher
	_, err = r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"bash"}, matched)

	// "file_read" matches second matcher
	matched = nil
	_, err = r.RunPreToolUse(context.Background(), "s", "file_read", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"file_read"}, matched)

	// "curl" matches neither
	matched = nil
	_, err = r.RunPreToolUse(context.Background(), "s", "curl", nil)
	require.NoError(t, err)
	assert.Empty(t, matched)
}

func TestEmptyPatternMatchesAll(t *testing.T) {
	called := 0
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					called++
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	r.RunPreToolUse(context.Background(), "s", "bash", nil)
	r.RunPreToolUse(context.Background(), "s", "curl", nil)
	r.RunPreToolUse(context.Background(), "s", "anything", nil)
	assert.Equal(t, 3, called, "empty pattern should match all tool names")
}

func TestFirstBlockWins(t *testing.T) {
	thirdCalled := false
	r, err := hookrunner.New([]pubhook.Matcher{
		{Event: pubhook.PreToolUse, Hooks: []pubhook.Func{allowHook()}},
		{Event: pubhook.PreToolUse, Hooks: []pubhook.Func{blockHook("reason-1")}},
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					thirdCalled = true
					return &pubhook.Result{Block: true, Reason: "reason-2"}, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Block)
	assert.Equal(t, "reason-1", res.Reason, "first block reason wins")
	assert.False(t, thirdCalled, "hooks after block should not execute")
}

func TestFirstBlockWinsWithinMatcher(t *testing.T) {
	secondCalled := false
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				blockHook("inner-block"),
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					secondCalled = true
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Block)
	assert.Equal(t, "inner-block", res.Reason)
	assert.False(t, secondCalled)
}

func TestTimeoutEnforcement(t *testing.T) {
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event:   pubhook.PreToolUse,
			Timeout: 50 * time.Millisecond,
			Hooks: []pubhook.Func{
				func(ctx context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case <-time.After(5 * time.Second):
						return nil, nil
					}
				},
			},
		},
	})
	require.NoError(t, err)

	start := time.Now()
	_, err = r.RunPreToolUse(context.Background(), "s", "bash", nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.True(t, elapsed < 2*time.Second, "should timeout quickly, took %v", elapsed)
}

func TestUpdatedInputPropagation(t *testing.T) {
	input1 := json.RawMessage(`{"v":1}`)
	input2 := json.RawMessage(`{"v":2}`)

	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					return &pubhook.Result{UpdatedInput: input1}, nil
				},
			},
		},
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					return &pubhook.Result{UpdatedInput: input2}, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.JSONEq(t, `{"v":2}`, string(res.UpdatedInput), "last non-nil UpdatedInput wins")
}

func TestRunPostToolUse(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PostToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	inputJSON := json.RawMessage(`{"cmd":"ls"}`)
	err = r.RunPostToolUse(context.Background(), "sess-2", "bash", inputJSON, "file1\nfile2")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PostToolUse, captured.Event)
	assert.Equal(t, "bash", captured.ToolName)
	assert.Equal(t, "file1\nfile2", captured.ToolOutput)
	assert.JSONEq(t, `{"cmd":"ls"}`, string(captured.ToolInput))
}

func TestRunPostToolFailure(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PostToolUseFailure,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	toolErr := errors.New("command not found")
	err = r.RunPostToolFailure(context.Background(), "sess-3", "bash", nil, toolErr)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PostToolUseFailure, captured.Event)
	assert.Equal(t, "bash", captured.ToolName)
	assert.Equal(t, toolErr, captured.ToolError)
}

func TestRunStop(t *testing.T) {
	called := false
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.Stop,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					called = true
					assert.Equal(t, pubhook.Stop, in.Event)
					assert.Equal(t, "sess-4", in.SessionID)
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunStop(context.Background(), "sess-4")
	require.NoError(t, err)
	assert.True(t, called)
}

func TestMultipleMatchersForSameEvent(t *testing.T) {
	var order []int
	makeHook := func(id int) pubhook.Func {
		return func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
			order = append(order, id)
			return nil, nil
		}
	}

	r, err := hookrunner.New([]pubhook.Matcher{
		{Event: pubhook.PreToolUse, Hooks: []pubhook.Func{makeHook(1)}},
		{Event: pubhook.PreToolUse, Hooks: []pubhook.Func{makeHook(2)}},
		{Event: pubhook.PreToolUse, Hooks: []pubhook.Func{makeHook(3)}},
	})
	require.NoError(t, err)

	_, err = r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, order, "hooks should execute in registration order")
}

func TestHookErrorStopsExecution(t *testing.T) {
	secondCalled := false
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					return nil, errors.New("hook failed")
				},
			},
		},
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					secondCalled = true
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.Error(t, err)
	assert.Equal(t, "hook failed", err.Error())
	assert.False(t, secondCalled)
}

func TestDecisionPropagation(t *testing.T) {
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					return &pubhook.Result{Decision: "allow"}, nil
				},
			},
		},
		{
			Event: pubhook.PreToolUse,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					return &pubhook.Result{Decision: "deny"}, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunPreToolUse(context.Background(), "s", "bash", nil)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, "deny", res.Decision, "last decision wins")
}

func TestRunSessionStart(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.SessionStart,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunSessionStart(context.Background(), "sess-start")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.SessionStart, captured.Event)
	assert.Equal(t, "sess-start", captured.SessionID)
}

func TestRunSessionEnd(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.SessionEnd,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunSessionEnd(context.Background(), "sess-end")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.SessionEnd, captured.Event)
	assert.Equal(t, "sess-end", captured.SessionID)
}

func TestRunPreCompact(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreCompact,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunPreCompact(context.Background(), "sess-c", "server")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PreCompact, captured.Event)
	assert.Equal(t, "server", captured.CompactStrategy)
}

func TestRunPostCompact(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PostCompact,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunPostCompact(context.Background(), "sess-c", "client")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PostCompact, captured.Event)
	assert.Equal(t, "client", captured.CompactStrategy)
}

func TestRunPreAPIRequest(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PreAPIRequest,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunPreAPIRequest(context.Background(), "sess-api", "claude-opus-4-6", 5)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PreAPIRequest, captured.Event)
	assert.Equal(t, "claude-opus-4-6", captured.Model)
	assert.Equal(t, 5, captured.MessageCount)
}

func TestRunPostAPIRequest(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PostAPIRequest,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunPostAPIRequest(context.Background(), "sess-api", "claude-opus-4-6", 100, 50)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PostAPIRequest, captured.Event)
	assert.Equal(t, "claude-opus-4-6", captured.Model)
	assert.Equal(t, int64(100), captured.InputTokens)
	assert.Equal(t, int64(50), captured.OutputTokens)
}

func TestRunToolResult(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event:   pubhook.ToolResult,
			Pattern: `^bash$`,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	inputJSON := json.RawMessage(`{"cmd":"ls"}`)
	err = r.RunToolResult(context.Background(), "sess-tr", "bash", inputJSON, "file.txt", false)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.ToolResult, captured.Event)
	assert.Equal(t, "bash", captured.ToolName)
	assert.Equal(t, "file.txt", captured.ToolOutput)
	assert.Nil(t, captured.ToolError)
	assert.JSONEq(t, `{"cmd":"ls"}`, string(captured.ToolInput))
}

func TestRunToolResult_Error(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.ToolResult,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunToolResult(context.Background(), "sess-tr", "bash", nil, "command not found", true)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.ToolResult, captured.Event)
	assert.Equal(t, "command not found", captured.ToolOutput)
	assert.NotNil(t, captured.ToolError)
	assert.Equal(t, "command not found", captured.ToolError.Error())
}

func TestRunNotification(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.Notification,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	payload := json.RawMessage(`{"msg":"hello"}`)
	err = r.RunNotification(context.Background(), "sess-n", "info", payload)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.Notification, captured.Event)
	assert.Equal(t, "info", captured.NotificationType)
	assert.JSONEq(t, `{"msg":"hello"}`, string(captured.Payload))
}

func TestRunUserPromptSubmit(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.UserPromptSubmit,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	res, err := r.RunUserPromptSubmit(context.Background(), "sess-ups", "hello world")
	require.NoError(t, err)
	assert.Nil(t, res, "no result when hook returns nil")
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.UserPromptSubmit, captured.Event)
	assert.Equal(t, "sess-ups", captured.SessionID)
	assert.Equal(t, "hello world", captured.Prompt)
}

func TestRunSubagentStart(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.SubagentStart,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunSubagentStart(context.Background(), "sess-sa", "researcher", "run-001")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.SubagentStart, captured.Event)
	assert.Equal(t, "sess-sa", captured.SessionID)
	assert.Equal(t, "researcher", captured.AgentName)
	assert.Equal(t, "run-001", captured.RunID)
}

func TestRunSubagentStop(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.SubagentStop,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return nil, nil
				},
			},
		},
	})
	require.NoError(t, err)

	err = r.RunSubagentStop(context.Background(), "sess-sa", "researcher", "run-001")
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.SubagentStop, captured.Event)
	assert.Equal(t, "sess-sa", captured.SessionID)
	assert.Equal(t, "researcher", captured.AgentName)
	assert.Equal(t, "run-001", captured.RunID)
}

func TestRunPermissionRequest(t *testing.T) {
	var captured *pubhook.Input
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event:   pubhook.PermissionRequest,
			Pattern: `^bash$`,
			Hooks: []pubhook.Func{
				func(_ context.Context, in *pubhook.Input) (*pubhook.Result, error) {
					captured = in
					return &pubhook.Result{Decision: "allow"}, nil
				},
			},
		},
	})
	require.NoError(t, err)

	inputJSON := json.RawMessage(`{"cmd":"rm -rf /"}`)
	res, err := r.RunPermissionRequest(context.Background(), "sess-perm", "bash", inputJSON)
	require.NoError(t, err)
	require.NotNil(t, captured)
	assert.Equal(t, pubhook.PermissionRequest, captured.Event)
	assert.Equal(t, "sess-perm", captured.SessionID)
	assert.Equal(t, "bash", captured.ToolName)
	assert.JSONEq(t, `{"cmd":"rm -rf /"}`, string(captured.ToolInput))
	require.NotNil(t, res)
	assert.Equal(t, "allow", res.Decision)
}

func TestRunPermissionRequest_Block(t *testing.T) {
	r, err := hookrunner.New([]pubhook.Matcher{
		{
			Event: pubhook.PermissionRequest,
			Hooks: []pubhook.Func{
				func(_ context.Context, _ *pubhook.Input) (*pubhook.Result, error) {
					return &pubhook.Result{Block: true, Reason: "dangerous command"}, nil
				},
			},
		},
	})
	require.NoError(t, err)

	inputJSON := json.RawMessage(`{"cmd":"rm -rf /"}`)
	res, err := r.RunPermissionRequest(context.Background(), "sess-perm", "bash", inputJSON)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.Block)
	assert.Equal(t, "dangerous command", res.Reason)
}
