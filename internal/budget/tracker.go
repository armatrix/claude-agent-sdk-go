package budget

import (
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
)

// MaxDecimal is a sentinel value representing an effectively unlimited remaining budget.
var MaxDecimal = decimal.New(1, 18) // 1e18

// Usage holds token counts for a single API call.
type Usage struct {
	InputTokens              int
	OutputTokens             int
	CacheReadInputTokens     int
	CacheCreationInputTokens int
}

// UsageIteration represents one sampling step (compaction or message).
type UsageIteration struct {
	Type  string // "compaction" | "message"
	Usage Usage
}

// BudgetTracker tracks cumulative token usage and cost across API calls.
// It is safe for concurrent use.
type BudgetTracker struct {
	maxBudget  decimal.Decimal // 0 = unlimited
	totalCost  decimal.Decimal
	totalUsage Usage
	pricing    map[anthropic.Model]ModelPricing
	mu         sync.Mutex
}

// NewBudgetTracker creates a new tracker. maxBudget of 0 means unlimited.
func NewBudgetTracker(maxBudget decimal.Decimal, pricing map[anthropic.Model]ModelPricing) *BudgetTracker {
	return &BudgetTracker{
		maxBudget: maxBudget,
		totalCost: decimal.Zero,
		pricing:   pricing,
	}
}

// RecordUsage records token usage for a single API call and updates the cumulative cost.
func (b *BudgetTracker) RecordUsage(model anthropic.Model, usage Usage) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.totalUsage.InputTokens += usage.InputTokens
	b.totalUsage.OutputTokens += usage.OutputTokens
	b.totalUsage.CacheReadInputTokens += usage.CacheReadInputTokens
	b.totalUsage.CacheCreationInputTokens += usage.CacheCreationInputTokens

	pricing, ok := b.pricing[model]
	if !ok {
		return // Unknown model — tokens counted but no cost added
	}

	totalInput := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
	inputCost := pricing.CostForInput(usage.InputTokens, usage.CacheReadInputTokens, usage.CacheCreationInputTokens, totalInput)
	outputCost := pricing.CostForOutput(usage.OutputTokens, totalInput)

	b.totalCost = b.totalCost.Add(inputCost).Add(outputCost)
}

// RecordIterations records multiple usage iterations (e.g. compaction + message steps).
func (b *BudgetTracker) RecordIterations(model anthropic.Model, iterations []UsageIteration) {
	for _, iter := range iterations {
		b.RecordUsage(model, iter.Usage)
	}
}

// TotalCost returns the cumulative cost across all recorded usage.
func (b *BudgetTracker) TotalCost() decimal.Decimal {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalCost
}

// TotalUsage returns the cumulative token usage across all recorded calls.
func (b *BudgetTracker) TotalUsage() Usage {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.totalUsage
}

// Remaining returns the remaining budget. If maxBudget is 0 (unlimited), returns MaxDecimal.
func (b *BudgetTracker) Remaining() decimal.Decimal {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.maxBudget.IsZero() {
		return MaxDecimal
	}
	return b.maxBudget.Sub(b.totalCost)
}

// Exhausted returns true if the total cost has reached or exceeded maxBudget.
// Always returns false if maxBudget is 0 (unlimited).
func (b *BudgetTracker) Exhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.maxBudget.IsZero() {
		return false
	}
	return b.totalCost.GreaterThanOrEqual(b.maxBudget)
}
