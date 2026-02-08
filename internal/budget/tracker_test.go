package budget

import (
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCostForInput_StandardPricing(t *testing.T) {
	p := DefaultPricing[anthropic.ModelClaudeOpus4_6]

	// 1000 input tokens at $5/MTok = $0.005
	cost := p.CostForInput(1000, 0, 0, 1000)
	expected := decimal.NewFromFloat(0.005)
	assert.True(t, expected.Equal(cost), "expected %s, got %s", expected, cost)
}

func TestCostForOutput_StandardPricing(t *testing.T) {
	p := DefaultPricing[anthropic.ModelClaudeOpus4_6]

	// 500 output tokens at $25/MTok = $0.0125
	cost := p.CostForOutput(500, 1000)
	expected := decimal.NewFromFloat(0.0125)
	assert.True(t, expected.Equal(cost), "expected %s, got %s", expected, cost)
}

func TestCostForInput_LongContext(t *testing.T) {
	p := DefaultPricing[anthropic.ModelClaudeOpus4_6]

	// 250K total input → long context pricing applies to ALL tokens
	// 250000 input at $10/MTok = $2.50
	cost := p.CostForInput(250_000, 0, 0, 250_000)
	expected := decimal.NewFromFloat(2.5)
	assert.True(t, expected.Equal(cost), "expected %s, got %s", expected, cost)
}

func TestCostForOutput_LongContext(t *testing.T) {
	p := DefaultPricing[anthropic.ModelClaudeOpus4_6]

	// Output with 250K input → long output rate
	// 1000 output at $37.50/MTok = $0.0375
	cost := p.CostForOutput(1000, 250_000)
	expected := decimal.NewFromFloat(0.0375)
	assert.True(t, expected.Equal(cost), "expected %s, got %s", expected, cost)
}

func TestCostForInput_CacheTokens(t *testing.T) {
	p := DefaultPricing[anthropic.ModelClaudeOpus4_6]

	// 500 input + 200 cache read + 300 cache write, total 1000 (under threshold)
	// input:     500 * $5/MTok   = $0.0025
	// cacheRead: 200 * $0.50/MTok = $0.0001
	// cacheWrite:300 * $6.25/MTok = $0.001875
	cost := p.CostForInput(500, 200, 300, 1000)
	expected := decimal.NewFromFloat(0.0025).
		Add(decimal.NewFromFloat(0.0001)).
		Add(decimal.NewFromFloat(0.001875))
	assert.True(t, expected.Equal(cost), "expected %s, got %s", expected, cost)
}

func TestCostForInput_HaikuNoLongContext(t *testing.T) {
	p := DefaultPricing[anthropic.ModelClaudeHaiku4_5]

	// Haiku has LongContextThreshold=0, so it never triggers long pricing
	// Even with 500K total input, standard rate applies
	// 500000 * $1/MTok = $0.50
	cost := p.CostForInput(500_000, 0, 0, 500_000)
	expected := decimal.NewFromFloat(0.5)
	assert.True(t, expected.Equal(cost), "expected %s, got %s", expected, cost)
}

func TestRecordUsage_StandardOpus(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	})

	// input: 1000 * $5/MTok = $0.005
	// output: 500 * $25/MTok = $0.0125
	// total = $0.0175
	expected := decimal.NewFromFloat(0.0175)
	assert.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())

	usage := bt.TotalUsage()
	assert.Equal(t, 1000, usage.InputTokens)
	assert.Equal(t, 500, usage.OutputTokens)
}

func TestRecordUsage_LongContextAllTokensPremium(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{
		InputTokens:  250_000,
		OutputTokens: 1000,
	})

	// Long context: totalInput=250000 > 200000
	// input: 250000 * $10/MTok = $2.50
	// output: 1000 * $37.50/MTok = $0.0375
	// total = $2.5375
	expected := decimal.NewFromFloat(2.5375)
	assert.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())
}

func TestRecordUsage_WithCacheTokens(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	bt.RecordUsage(anthropic.ModelClaudeSonnet4_5, Usage{
		InputTokens:              5000,
		OutputTokens:             2000,
		CacheReadInputTokens:     1000,
		CacheCreationInputTokens: 500,
	})

	// totalInput = 5000 + 1000 + 500 = 6500 (under 200K → standard rates)
	// input:      5000 * $3/MTok   = $0.015
	// cacheRead:  1000 * $0.30/MTok = $0.0003
	// cacheWrite: 500  * $3.75/MTok = $0.001875
	// output:     2000 * $15/MTok  = $0.030
	// total = $0.047175
	expected := decimal.NewFromFloat(0.015).
		Add(decimal.NewFromFloat(0.0003)).
		Add(decimal.NewFromFloat(0.001875)).
		Add(decimal.NewFromFloat(0.030))
	assert.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())
}

func TestBudgetUnlimited(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{
		InputTokens:  1_000_000,
		OutputTokens: 500_000,
	})

	assert.False(t, bt.Exhausted(), "unlimited budget should never be exhausted")
	assert.True(t, MaxDecimal.Equal(bt.Remaining()), "remaining should be MaxDecimal for unlimited budget")
}

func TestBudgetExhaustion(t *testing.T) {
	// Set a tiny budget of $0.01
	bt := NewBudgetTracker(decimal.NewFromFloat(0.01), DefaultPricing)

	assert.False(t, bt.Exhausted(), "should not be exhausted before usage")

	// 1000 input + 500 output on opus → $0.0175 > $0.01
	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	})

	assert.True(t, bt.Exhausted(), "should be exhausted after exceeding budget")
}

func TestRemaining(t *testing.T) {
	bt := NewBudgetTracker(decimal.NewFromFloat(1.0), DefaultPricing)

	// 1000 input on opus → $0.005
	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{InputTokens: 1000})

	expected := decimal.NewFromFloat(0.995)
	remaining := bt.Remaining()
	assert.True(t, expected.Equal(remaining), "expected remaining %s, got %s", expected, remaining)
}

func TestRecordIterations(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	iterations := []UsageIteration{
		{
			Type:  "compaction",
			Usage: Usage{InputTokens: 100_000, OutputTokens: 5000},
		},
		{
			Type:  "message",
			Usage: Usage{InputTokens: 2000, OutputTokens: 1000},
		},
	}

	bt.RecordIterations(anthropic.ModelClaudeOpus4_6, iterations)

	usage := bt.TotalUsage()
	assert.Equal(t, 102_000, usage.InputTokens)
	assert.Equal(t, 6000, usage.OutputTokens)

	// compaction: input 100000*$5/MTok=$0.50, output 5000*$25/MTok=$0.125 → $0.625
	// message:    input 2000*$5/MTok=$0.01, output 1000*$25/MTok=$0.025 → $0.035
	// total: $0.66
	expected := decimal.NewFromFloat(0.66)
	assert.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())
}

func TestRecordUsage_UnknownModel(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	bt.RecordUsage("unknown-model", Usage{
		InputTokens:  1000,
		OutputTokens: 500,
	})

	// Tokens counted but no cost added
	usage := bt.TotalUsage()
	assert.Equal(t, 1000, usage.InputTokens)
	assert.Equal(t, 500, usage.OutputTokens)
	assert.True(t, decimal.Zero.Equal(bt.TotalCost()), "unknown model should not add cost")
}

func TestConcurrentAccess(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	var wg sync.WaitGroup
	goroutines := 100

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{
				InputTokens:  1000,
				OutputTokens: 500,
			})
		}()
	}

	wg.Wait()

	usage := bt.TotalUsage()
	assert.Equal(t, goroutines*1000, usage.InputTokens)
	assert.Equal(t, goroutines*500, usage.OutputTokens)

	// Each goroutine: $0.0175, total = 100 * $0.0175 = $1.75
	expected := decimal.NewFromFloat(0.0175).Mul(decimal.NewFromInt(int64(goroutines)))
	require.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())
}

func TestMultipleRecordUsageCumulative(t *testing.T) {
	bt := NewBudgetTracker(decimal.NewFromFloat(10.0), DefaultPricing)

	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{InputTokens: 1000})
	bt.RecordUsage(anthropic.ModelClaudeSonnet4_5, Usage{OutputTokens: 2000})
	bt.RecordUsage(anthropic.ModelClaudeHaiku4_5, Usage{InputTokens: 500, OutputTokens: 500})

	usage := bt.TotalUsage()
	assert.Equal(t, 1500, usage.InputTokens)
	assert.Equal(t, 2500, usage.OutputTokens)

	// opus input:  1000 * $5/MTok = $0.005
	// sonnet output: 2000 * $15/MTok = $0.030
	// haiku input: 500 * $1/MTok = $0.0005, output: 500 * $5/MTok = $0.0025
	// total = $0.005 + $0.030 + $0.0005 + $0.0025 = $0.038
	expected := decimal.NewFromFloat(0.038)
	assert.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())

	assert.False(t, bt.Exhausted())
	expectedRemaining := decimal.NewFromFloat(9.962)
	assert.True(t, expectedRemaining.Equal(bt.Remaining()), "expected remaining %s, got %s", expectedRemaining, bt.Remaining())
}

func TestExhaustedExact(t *testing.T) {
	// Budget exactly equals cost
	// 1000 input on opus = $0.005
	bt := NewBudgetTracker(decimal.NewFromFloat(0.005), DefaultPricing)
	bt.RecordUsage(anthropic.ModelClaudeOpus4_6, Usage{InputTokens: 1000})

	assert.True(t, bt.Exhausted(), "should be exhausted when cost equals budget exactly")
	assert.True(t, decimal.Zero.Equal(bt.Remaining()), "remaining should be zero")
}

func TestLongContextWithCache(t *testing.T) {
	bt := NewBudgetTracker(decimal.Zero, DefaultPricing)

	// Total input = 150000 + 60000 + 10000 = 220000 > 200K → long context
	bt.RecordUsage(anthropic.ModelClaudeSonnet4_5, Usage{
		InputTokens:              150_000,
		OutputTokens:             5000,
		CacheReadInputTokens:     60_000,
		CacheCreationInputTokens: 10_000,
	})

	// Long context triggered (220K > 200K)
	// input:      150000 * $6/MTok   = $0.90
	// cacheRead:  60000  * $0.30/MTok = $0.018
	// cacheWrite: 10000  * $3.75/MTok = $0.0375
	// output:     5000   * $22.50/MTok = $0.1125
	// total = $1.068
	expected := decimal.NewFromFloat(0.9).
		Add(decimal.NewFromFloat(0.018)).
		Add(decimal.NewFromFloat(0.0375)).
		Add(decimal.NewFromFloat(0.1125))
	assert.True(t, expected.Equal(bt.TotalCost()), "expected %s, got %s", expected, bt.TotalCost())
}
