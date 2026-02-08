// Package hook defines public types for the agent hook system.
//
// Hooks let users register callbacks that fire before/after tool execution,
// at session boundaries, and around API requests. The [Matcher] type binds
// a set of [Func] callbacks to a specific [Event] and an optional tool-name
// regex pattern.
package hook

import (
	"context"
	"encoding/json"
	"time"
)

// Event identifies when a hook fires.
type Event string

const (
	PreToolUse         Event = "PreToolUse"
	PostToolUse        Event = "PostToolUse"
	PostToolUseFailure Event = "PostToolUseFailure"
	Stop               Event = "Stop"
	SessionStart       Event = "SessionStart"
	SessionEnd         Event = "SessionEnd"
	PreCompact         Event = "PreCompact"
	PostCompact        Event = "PostCompact"
	PreAPIRequest      Event = "PreAPIRequest"
	PostAPIRequest     Event = "PostAPIRequest"
	ToolResult         Event = "ToolResult"
	Notification       Event = "Notification"
)

// Input is passed to hook functions.
type Input struct {
	SessionID  string
	Event      Event
	ToolName   string          // Only for tool-related events.
	ToolInput  json.RawMessage // Only for PreToolUse.
	ToolOutput string          // Only for PostToolUse.
	ToolError  error           // Only for PostToolUseFailure.
}

// Result is returned by hook functions. A zero value means "no action".
type Result struct {
	Block        bool            // If true, blocks the tool from executing.
	Reason       string          // Human-readable reason for blocking.
	UpdatedInput json.RawMessage // If non-nil, replaces the tool input (PreToolUse only).
	Decision     string          // "allow", "deny", "ask" — for permission hooks.
}

// Func is the signature for hook callbacks.
type Func func(ctx context.Context, input *Input) (*Result, error)

// Matcher defines which events a set of hooks should fire for.
type Matcher struct {
	Event   Event         // Which event to match.
	Pattern string        // Regex pattern for tool name (empty = match all).
	Hooks   []Func        // Functions to call (in order).
	Timeout time.Duration // Max time for all hooks in this matcher (0 = 30s default).
}
