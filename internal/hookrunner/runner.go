// Package hookrunner provides the internal runner that executes hook matchers.
package hookrunner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	pubhook "github.com/armatrix/claude-agent-sdk-go/hook"
)

const defaultTimeout = 30 * time.Second

// Runner executes hooks matched by event and tool name.
type Runner struct {
	matchers []matcherEntry
}

type matcherEntry struct {
	event   pubhook.Event
	pattern *regexp.Regexp // nil = match all tools
	hooks   []pubhook.Func
	timeout time.Duration
}

// New creates a Runner from public Matcher definitions.
// Returns an error if any regex pattern is invalid.
func New(matchers []pubhook.Matcher) (*Runner, error) {
	entries := make([]matcherEntry, 0, len(matchers))
	for i, m := range matchers {
		entry := matcherEntry{
			event:   m.Event,
			hooks:   m.Hooks,
			timeout: m.Timeout,
		}
		if entry.timeout == 0 {
			entry.timeout = defaultTimeout
		}
		if m.Pattern != "" {
			re, err := regexp.Compile(m.Pattern)
			if err != nil {
				return nil, fmt.Errorf("matcher[%d]: invalid pattern %q: %w", i, m.Pattern, err)
			}
			entry.pattern = re
		}
		entries = append(entries, entry)
	}
	return &Runner{matchers: entries}, nil
}

// RunPreToolUse runs all matching PreToolUse hooks. Returns the combined result.
// First block wins. UpdatedInput from the last non-nil update wins.
func (r *Runner) RunPreToolUse(ctx context.Context, sessionID, toolName string, input json.RawMessage) (*pubhook.Result, error) {
	return r.run(ctx, pubhook.PreToolUse, sessionID, toolName, &pubhook.Input{
		SessionID: sessionID,
		Event:     pubhook.PreToolUse,
		ToolName:  toolName,
		ToolInput: input,
	})
}

// RunPostToolUse runs all matching PostToolUse hooks.
func (r *Runner) RunPostToolUse(ctx context.Context, sessionID, toolName string, input json.RawMessage, output string) error {
	_, err := r.run(ctx, pubhook.PostToolUse, sessionID, toolName, &pubhook.Input{
		SessionID:  sessionID,
		Event:      pubhook.PostToolUse,
		ToolName:   toolName,
		ToolInput:  input,
		ToolOutput: output,
	})
	return err
}

// RunPostToolFailure runs all matching PostToolUseFailure hooks.
func (r *Runner) RunPostToolFailure(ctx context.Context, sessionID, toolName string, input json.RawMessage, toolErr error) error {
	_, err := r.run(ctx, pubhook.PostToolUseFailure, sessionID, toolName, &pubhook.Input{
		SessionID: sessionID,
		Event:     pubhook.PostToolUseFailure,
		ToolName:  toolName,
		ToolInput: input,
		ToolError: toolErr,
	})
	return err
}

// RunStop runs all matching Stop hooks.
func (r *Runner) RunStop(ctx context.Context, sessionID string) error {
	_, err := r.run(ctx, pubhook.Stop, sessionID, "", &pubhook.Input{
		SessionID: sessionID,
		Event:     pubhook.Stop,
	})
	return err
}

// run is the internal dispatcher.
func (r *Runner) run(ctx context.Context, event pubhook.Event, sessionID, toolName string, input *pubhook.Input) (*pubhook.Result, error) {
	var combined *pubhook.Result

	for _, entry := range r.matchers {
		if entry.event != event {
			continue
		}
		if entry.pattern != nil && !entry.pattern.MatchString(toolName) {
			continue
		}

		tctx, cancel := context.WithTimeout(ctx, entry.timeout)
		res, err := runHooks(tctx, entry.hooks, input)
		cancel()

		if err != nil {
			return combined, err
		}
		if res == nil {
			continue
		}

		if combined == nil {
			combined = &pubhook.Result{}
		}

		if res.Block && !combined.Block {
			combined.Block = true
			combined.Reason = res.Reason
		}
		if res.UpdatedInput != nil {
			combined.UpdatedInput = res.UpdatedInput
		}
		if res.Decision != "" {
			combined.Decision = res.Decision
		}

		if combined.Block {
			break
		}
	}

	return combined, nil
}

// runHooks executes a slice of hook functions in order.
// It stops early if a hook blocks or the context is cancelled.
func runHooks(ctx context.Context, hooks []pubhook.Func, input *pubhook.Input) (*pubhook.Result, error) {
	var combined *pubhook.Result

	for _, fn := range hooks {
		if err := ctx.Err(); err != nil {
			return combined, err
		}

		res, err := fn(ctx, input)
		if err != nil {
			return combined, err
		}
		if res == nil {
			continue
		}

		if combined == nil {
			combined = &pubhook.Result{}
		}
		if res.Block {
			combined.Block = true
			combined.Reason = res.Reason
		}
		if res.UpdatedInput != nil {
			combined.UpdatedInput = res.UpdatedInput
		}
		if res.Decision != "" {
			combined.Decision = res.Decision
		}

		if combined.Block {
			return combined, nil
		}
	}

	return combined, nil
}
