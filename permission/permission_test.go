package permission_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armatrix/claude-agent-sdk-go/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModeDefault(t *testing.T) {
	checker := permission.NewChecker(permission.ModeDefault, nil)
	ctx := context.Background()

	// Read-only tools are allowed
	for _, tool := range []string{"Read", "Glob", "Grep", "WebFetch", "WebSearch"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Allow, d, "read tool %s should be allowed", tool)
	}

	// Write tools require asking
	for _, tool := range []string{"Write", "Edit"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Ask, d, "write tool %s should ask", tool)
	}

	// Bash requires asking
	d, err := checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d)
}

func TestModeAcceptEdits(t *testing.T) {
	checker := permission.NewChecker(permission.ModeAcceptEdits, nil)
	ctx := context.Background()

	// Read-only tools are allowed
	for _, tool := range []string{"Read", "Glob", "Grep"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Allow, d, "read tool %s should be allowed", tool)
	}

	// Write tools are allowed
	for _, tool := range []string{"Write", "Edit"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Allow, d, "write tool %s should be allowed", tool)
	}

	// Bash requires asking
	d, err := checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d)
}

func TestModeBypassPermissions(t *testing.T) {
	checker := permission.NewChecker(permission.ModeBypassPermissions, nil)
	ctx := context.Background()

	// Everything is allowed
	for _, tool := range []string{"Read", "Write", "Edit", "Bash", "Glob", "Grep", "UnknownTool"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Allow, d, "tool %s should be allowed in bypass mode", tool)
	}
}

func TestModePlan(t *testing.T) {
	checker := permission.NewChecker(permission.ModePlan, nil)
	ctx := context.Background()

	// Read-only tools are allowed
	for _, tool := range []string{"Read", "Glob", "Grep", "WebFetch", "WebSearch"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Allow, d, "read tool %s should be allowed in plan mode", tool)
	}

	// Write tools are denied
	for _, tool := range []string{"Write", "Edit"} {
		d, err := checker.Check(ctx, tool, nil)
		require.NoError(t, err)
		assert.Equal(t, permission.Deny, d, "write tool %s should be denied in plan mode", tool)
	}

	// Bash is denied
	d, err := checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Deny, d)
}

func TestCustomCanUseTool(t *testing.T) {
	// Custom callback that always denies
	alwaysDeny := func(ctx context.Context, toolName string, input json.RawMessage) (permission.Decision, error) {
		return permission.Deny, nil
	}

	checker := permission.NewChecker(permission.ModeBypassPermissions, alwaysDeny)
	ctx := context.Background()

	// Even in bypass mode, the callback overrides
	d, err := checker.Check(ctx, "Read", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Deny, d, "custom callback should override mode")

	d, err = checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Deny, d, "custom callback should override mode for Bash")
}

func TestSetMode(t *testing.T) {
	checker := permission.NewChecker(permission.ModeDefault, nil)
	ctx := context.Background()

	// Default: Write asks
	d, err := checker.Check(ctx, "Write", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d)

	// Switch to AcceptEdits: Write allowed
	checker.SetMode(permission.ModeAcceptEdits)
	assert.Equal(t, permission.ModeAcceptEdits, checker.Mode())

	d, err = checker.Check(ctx, "Write", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Allow, d)

	// Switch to Plan: Write denied
	checker.SetMode(permission.ModePlan)
	d, err = checker.Check(ctx, "Write", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Deny, d)
}

func TestUnknownToolFallthrough(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		mode     permission.Mode
		expected permission.Decision
	}{
		{permission.ModeDefault, permission.Ask},
		{permission.ModeAcceptEdits, permission.Ask},
		{permission.ModeBypassPermissions, permission.Allow},
		{permission.ModePlan, permission.Deny},
	}

	for _, tt := range tests {
		checker := permission.NewChecker(tt.mode, nil)
		d, err := checker.Check(ctx, "SomeUnknownTool", nil)
		require.NoError(t, err)
		assert.Equal(t, tt.expected, d, "unknown tool in mode %d", tt.mode)
	}
}

func TestNilCanUseToolUsesMode(t *testing.T) {
	checker := permission.NewChecker(permission.ModeDefault, nil)
	ctx := context.Background()

	// Should use mode-based logic, not panic
	d, err := checker.Check(ctx, "Read", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Allow, d)

	d, err = checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d)
}
