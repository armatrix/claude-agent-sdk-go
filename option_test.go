package agent

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOptionsDefaults(t *testing.T) {
	opts := resolveOptions(nil)

	assert.Equal(t, DefaultModel, opts.model)
	assert.Equal(t, DefaultContextWindow, opts.contextWindow)
	assert.Equal(t, DefaultMaxOutputTokens, opts.maxOutputTokens)
	assert.Equal(t, DefaultMaxTurns, opts.maxTurns)
	assert.Equal(t, DefaultCompactTriggerTokens, opts.compact.TriggerTokens)
	assert.Equal(t, 2, opts.compact.PreserveLastN)
	assert.Equal(t, DefaultStreamBufferSize, opts.streamBufferSize)
	assert.True(t, opts.maxBudget.IsZero())
}

func TestWithModel(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithModel(anthropic.ModelClaudeSonnet4_5),
	})
	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, opts.model)
}

func TestWithContextWindow(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithContextWindow(ContextWindow1M),
	})
	assert.Equal(t, ContextWindow1M, opts.contextWindow)
}

func TestWithMaxOutputTokens(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithMaxOutputTokens(MaxOutputTokensOpus46),
	})
	assert.Equal(t, MaxOutputTokensOpus46, opts.maxOutputTokens)
}

func TestWithMaxTurns(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithMaxTurns(10),
	})
	assert.Equal(t, 10, opts.maxTurns)
}

func TestWithBudget(t *testing.T) {
	budget := decimal.NewFromFloat(5.0)
	opts := resolveOptions([]AgentOption{
		WithBudget(budget),
	})
	assert.True(t, budget.Equal(opts.maxBudget))
}

func TestWithCompaction(t *testing.T) {
	config := CompactConfig{
		Strategy:          CompactClient,
		TriggerTokens:     100_000,
		PauseAfterCompact: true,
		PreserveLastN:     5,
	}
	opts := resolveOptions([]AgentOption{
		WithCompaction(config),
	})
	assert.Equal(t, CompactClient, opts.compact.Strategy)
	assert.Equal(t, 100_000, opts.compact.TriggerTokens)
	assert.True(t, opts.compact.PauseAfterCompact)
	assert.Equal(t, 5, opts.compact.PreserveLastN)
}

func TestWithCompactTrigger(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithCompactTrigger(80_000),
	})
	assert.Equal(t, 80_000, opts.compact.TriggerTokens)
}

func TestWithCompactTriggerClampedToMin(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithCompactTrigger(10_000),
	})
	assert.Equal(t, MinCompactTriggerTokens, opts.compact.TriggerTokens)
}

func TestWithCompactDisabled(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithCompactDisabled(),
	})
	assert.Equal(t, CompactDisabled, opts.compact.Strategy)
}

func TestWithBuiltinTools(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithBuiltinTools("Read", "Write", "Bash"),
	})
	assert.Equal(t, []string{"Read", "Write", "Bash"}, opts.builtinTools)
}

func TestWithDisabledTools(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithDisabledTools("Bash"),
	})
	assert.Equal(t, []string{"Bash"}, opts.disabledTools)
}

func TestMultipleOptions(t *testing.T) {
	budget := decimal.NewFromFloat(10.0)
	opts := resolveOptions([]AgentOption{
		WithModel(anthropic.ModelClaudeHaiku4_5),
		WithContextWindow(ContextWindow1M),
		WithMaxOutputTokens(MaxOutputTokensOpus46),
		WithMaxTurns(50),
		WithBudget(budget),
		WithCompactTrigger(120_000),
		WithBuiltinTools("Read", "Write"),
		WithDisabledTools("Bash"),
	})

	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, opts.model)
	assert.Equal(t, ContextWindow1M, opts.contextWindow)
	assert.Equal(t, MaxOutputTokensOpus46, opts.maxOutputTokens)
	assert.Equal(t, 50, opts.maxTurns)
	assert.True(t, budget.Equal(opts.maxBudget))
	assert.Equal(t, 120_000, opts.compact.TriggerTokens)
	assert.Equal(t, []string{"Read", "Write"}, opts.builtinTools)
	assert.Equal(t, []string{"Bash"}, opts.disabledTools)
}

func TestNewAgentAppliesOptions(t *testing.T) {
	agent := NewAgent(
		WithModel(anthropic.ModelClaudeSonnet4_5),
		WithMaxTurns(20),
	)

	require.NotNil(t, agent)
	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, agent.Model())
	assert.Equal(t, 20, agent.Options().maxTurns)
}

func TestNewAgentDefaults(t *testing.T) {
	agent := NewAgent()

	require.NotNil(t, agent)
	assert.Equal(t, DefaultModel, agent.Model())
	assert.Equal(t, DefaultContextWindow, agent.Options().contextWindow)
	assert.Equal(t, DefaultMaxOutputTokens, agent.Options().maxOutputTokens)
}
