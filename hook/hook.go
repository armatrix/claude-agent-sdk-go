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
	UserPromptSubmit   Event = "UserPromptSubmit"
	SubagentStart      Event = "SubagentStart"
	SubagentStop       Event = "SubagentStop"
	PermissionRequest  Event = "PermissionRequest"
)

// Input is passed to hook functions.
type Input struct {
	SessionID  string
	Event      Event
	ToolName   string          // Tool-related events.
	ToolInput  json.RawMessage // PreToolUse, PostToolUse, PostToolUseFailure, ToolResult.
	ToolOutput string          // PostToolUse, ToolResult.
	ToolError  error           // PostToolUseFailure, ToolResult.

	// API request hooks
	Model        string // PreAPIRequest, PostAPIRequest.
	MessageCount int    // PreAPIRequest (number of messages being sent).
	InputTokens  int64  // PostAPIRequest.
	OutputTokens int64  // PostAPIRequest.

	// Compaction hooks
	CompactStrategy string // PreCompact, PostCompact.

	// Notification hook
	NotificationType string          // Notification.
	Payload          json.RawMessage // Notification.

	// UserPromptSubmit hook
	Prompt string // The user's prompt text.

	// SubagentStart / SubagentStop hooks
	AgentName string // Name of the sub-agent.
	RunID     string // Unique run identifier.

	// PermissionRequest hook reuses ToolName + ToolInput fields.
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
