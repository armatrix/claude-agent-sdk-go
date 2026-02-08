package permission_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/armatrix/claude-agent-sdk-go/permission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchRules_DenyTakesPrecedence(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "Bash", Decision: permission.Allow},
		{Pattern: "Bash", Decision: permission.Ask},
		{Pattern: "Bash", Decision: permission.Deny},
	}

	d, matched := permission.MatchRules(rules, "Bash")
	assert.True(t, matched)
	assert.Equal(t, permission.Deny, d, "deny should take precedence over allow and ask")
}

func TestMatchRules_AskBeforeAllow(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "Edit", Decision: permission.Allow},
		{Pattern: "Edit", Decision: permission.Ask},
	}

	d, matched := permission.MatchRules(rules, "Edit")
	assert.True(t, matched)
	assert.Equal(t, permission.Ask, d, "ask should take precedence over allow")
}

func TestMatchRules_GlobPattern(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "mcp__*", Decision: permission.Allow},
		{Pattern: "Edit*", Decision: permission.Ask},
	}

	tests := []struct {
		name     string
		tool     string
		wantDec  permission.Decision
		wantMatch bool
	}{
		{"mcp wildcard match", "mcp__context7__query", permission.Allow, true},
		{"mcp wildcard match 2", "mcp__plugin_playwright", permission.Allow, true},
		{"Edit prefix match", "EditFile", permission.Ask, true},
		{"Edit exact match", "Edit", permission.Ask, true},
		{"no match", "Bash", permission.Allow, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, matched := permission.MatchRules(rules, tt.tool)
			assert.Equal(t, tt.wantMatch, matched)
			if matched {
				assert.Equal(t, tt.wantDec, d)
			}
		})
	}
}

func TestMatchRules_QuestionMarkPattern(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "Rea?", Decision: permission.Allow},
	}

	d, matched := permission.MatchRules(rules, "Read")
	assert.True(t, matched)
	assert.Equal(t, permission.Allow, d)

	_, matched = permission.MatchRules(rules, "Ready")
	assert.False(t, matched, "? matches exactly one character, not two")
}

func TestMatchRules_NoMatch(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "Bash", Decision: permission.Deny},
		{Pattern: "Edit", Decision: permission.Ask},
	}

	_, matched := permission.MatchRules(rules, "Read")
	assert.False(t, matched)
}

func TestMatchRules_Empty(t *testing.T) {
	_, matched := permission.MatchRules(nil, "Bash")
	assert.False(t, matched, "nil rules should not match")

	_, matched = permission.MatchRules([]permission.Rule{}, "Bash")
	assert.False(t, matched, "empty rules should not match")
}

func TestMatchRules_InvalidPattern(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "[invalid", Decision: permission.Allow},
	}

	_, matched := permission.MatchRules(rules, "anything")
	assert.False(t, matched, "invalid pattern should not match")
}

func TestMatchRules_AllowOnly(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "Read", Decision: permission.Allow},
	}

	d, matched := permission.MatchRules(rules, "Read")
	assert.True(t, matched)
	assert.Equal(t, permission.Allow, d)
}

func TestCheckerWithRules_RulesOverrideMode(t *testing.T) {
	// Mode is Plan (deny writes), but rules allow Edit
	rules := []permission.Rule{
		{Pattern: "Edit", Decision: permission.Allow},
	}
	checker := permission.NewCheckerWithRules(permission.ModePlan, rules, nil)
	ctx := context.Background()

	d, err := checker.Check(ctx, "Edit", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Allow, d, "rule should override plan mode's deny for Edit")
}

func TestCheckerWithRules_RulesOverrideBypass(t *testing.T) {
	// Mode is Bypass (allow all), but rules deny Bash
	rules := []permission.Rule{
		{Pattern: "Bash", Decision: permission.Deny},
	}
	checker := permission.NewCheckerWithRules(permission.ModeBypassPermissions, rules, nil)
	ctx := context.Background()

	d, err := checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Deny, d, "rule should override bypass mode for Bash")
}

func TestCheckerWithRules_FallsThruToMode(t *testing.T) {
	// Rules only cover Bash; Read should fall through to mode-based logic
	rules := []permission.Rule{
		{Pattern: "Bash", Decision: permission.Deny},
	}
	checker := permission.NewCheckerWithRules(permission.ModeDefault, rules, nil)
	ctx := context.Background()

	// Read not matched by rules, falls to ModeDefault => Allow
	d, err := checker.Check(ctx, "Read", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Allow, d, "unmatched tool should fall through to mode defaults")

	// Write not matched by rules, falls to ModeDefault => Ask
	d, err = checker.Check(ctx, "Write", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d, "unmatched tool should fall through to mode defaults")
}

func TestCheckerWithRules_FallsThruToFunc(t *testing.T) {
	// Rules only cover mcp__ tools; everything else falls to callback
	rules := []permission.Rule{
		{Pattern: "mcp__*", Decision: permission.Allow},
	}
	alwaysAsk := func(ctx context.Context, toolName string, input json.RawMessage) (permission.Decision, error) {
		return permission.Ask, nil
	}
	checker := permission.NewCheckerWithRules(permission.ModeDefault, rules, alwaysAsk)
	ctx := context.Background()

	// mcp tool matched by rule => Allow
	d, err := checker.Check(ctx, "mcp__context7__query", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Allow, d, "mcp tool should be allowed by rule")

	// Bash not matched by rule, falls to callback => Ask
	d, err = checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d, "unmatched tool should fall through to canUseTool")

	// Read not matched by rule, falls to callback => Ask (callback overrides mode)
	d, err = checker.Check(ctx, "Read", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d, "unmatched tool should use callback, not mode")
}

func TestCheckerWithRules_MultipleGlobPatterns(t *testing.T) {
	rules := []permission.Rule{
		{Pattern: "mcp__*", Decision: permission.Allow},
		{Pattern: "Bash", Decision: permission.Ask},
		{Pattern: "Write", Decision: permission.Deny},
	}
	checker := permission.NewCheckerWithRules(permission.ModeDefault, rules, nil)
	ctx := context.Background()

	tests := []struct {
		tool string
		want permission.Decision
	}{
		{"mcp__context7__query", permission.Allow},
		{"mcp__plugin_playwright__click", permission.Allow},
		{"Bash", permission.Ask},
		{"Write", permission.Deny},
		{"Read", permission.Allow},  // falls through to ModeDefault
		{"Edit", permission.Ask},    // falls through to ModeDefault
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			d, err := checker.Check(ctx, tt.tool, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.want, d)
		})
	}
}

func TestNewCheckerWithRules_NilRulesActsLikeNewChecker(t *testing.T) {
	checker := permission.NewCheckerWithRules(permission.ModeDefault, nil, nil)
	ctx := context.Background()

	d, err := checker.Check(ctx, "Read", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Allow, d)

	d, err = checker.Check(ctx, "Bash", nil)
	require.NoError(t, err)
	assert.Equal(t, permission.Ask, d)
}
