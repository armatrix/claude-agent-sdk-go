package permission

import (
	"context"
	"encoding/json"
)

// Decision represents the outcome of a permission check.
type Decision int

const (
	Allow Decision = iota // Tool execution is permitted
	Deny                  // Tool execution is blocked
	Ask                   // User should be prompted for confirmation
)

// Mode controls the default permission behavior.
type Mode int

const (
	ModeDefault           Mode = iota // read=allow, write/bash=ask
	ModeAcceptEdits                   // read+write=allow, bash=ask
	ModeBypassPermissions             // all=allow
	ModePlan                          // read=allow, write+bash=deny
)

// Func is a user-provided permission callback.
// It receives the tool name and input, returns a Decision.
type Func func(ctx context.Context, toolName string, input json.RawMessage) (Decision, error)

// ReadOnlyTools lists tools classified as read-only.
// These are always allowed in Default and AcceptEdits modes.
var ReadOnlyTools = map[string]bool{
	"Read":      true,
	"Glob":      true,
	"Grep":      true,
	"WebFetch":  true,
	"WebSearch": true,
}

// WriteTools lists tools classified as write operations.
// Allowed in AcceptEdits and BypassPermissions modes.
var WriteTools = map[string]bool{
	"Write": true,
	"Edit":  true,
}

// Checker evaluates whether a tool can be used.
type Checker struct {
	mode       Mode
	canUseTool Func // Optional user-provided callback, overrides mode-based check
}

// NewChecker creates a permission checker with the given mode.
func NewChecker(mode Mode, canUseTool Func) *Checker {
	return &Checker{mode: mode, canUseTool: canUseTool}
}

// Check evaluates whether the named tool with the given input is allowed.
// Returns Allow (0), Deny (1), or Ask (2).
func (c *Checker) Check(ctx context.Context, toolName string, input json.RawMessage) (Decision, error) {
	// If user provided a callback, use it first
	if c.canUseTool != nil {
		return c.canUseTool(ctx, toolName, input)
	}

	// Mode-based default behavior
	switch c.mode {
	case ModeBypassPermissions:
		return Allow, nil
	case ModePlan:
		if ReadOnlyTools[toolName] {
			return Allow, nil
		}
		return Deny, nil
	case ModeAcceptEdits:
		if ReadOnlyTools[toolName] || WriteTools[toolName] {
			return Allow, nil
		}
		return Ask, nil
	default: // ModeDefault
		if ReadOnlyTools[toolName] {
			return Allow, nil
		}
		return Ask, nil
	}
}

// Mode returns the current permission mode.
func (c *Checker) Mode() Mode {
	return c.mode
}

// SetMode updates the permission mode.
func (c *Checker) SetMode(mode Mode) {
	c.mode = mode
}
