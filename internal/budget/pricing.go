package budget

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
)

// ModelPricing holds per-model token prices in USD per million tokens.
type ModelPricing struct {
	InputPerMTok         decimal.Decimal
	OutputPerMTok        decimal.Decimal
	LongInputPerMTok     decimal.Decimal // Premium rate when total input > LongContextThreshold
	LongOutputPerMTok    decimal.Decimal
	CacheWritePerMTok    decimal.Decimal
	CacheReadPerMTok     decimal.Decimal
	LongContextThreshold int // Input token count that triggers long-context pricing; 0 = no long context support
}

var million = decimal.NewFromInt(1_000_000)

// CostForInput calculates the input cost considering long context threshold and cache tokens.
// inputTokens: non-cache input tokens billed at standard/long input rate.
// cacheReadTokens: tokens read from cache, billed at cache read rate.
// cacheWriteTokens: tokens written to cache, billed at cache write rate.
// totalInputTokens: total input tokens (used to determine if long context pricing applies).
func (p ModelPricing) CostForInput(inputTokens, cacheReadTokens, cacheWriteTokens, totalInputTokens int) decimal.Decimal {
	rate := p.InputPerMTok
	if p.LongContextThreshold > 0 && totalInputTokens > p.LongContextThreshold {
		rate = p.LongInputPerMTok
	}

	cost := decimal.NewFromInt(int64(inputTokens)).Mul(rate).Div(million)
	cost = cost.Add(decimal.NewFromInt(int64(cacheReadTokens)).Mul(p.CacheReadPerMTok).Div(million))
	cost = cost.Add(decimal.NewFromInt(int64(cacheWriteTokens)).Mul(p.CacheWritePerMTok).Div(million))

	return cost
}

// CostForOutput calculates the output cost considering long context threshold.
// totalInputTokens determines whether long context premium applies.
func (p ModelPricing) CostForOutput(outputTokens, totalInputTokens int) decimal.Decimal {
	rate := p.OutputPerMTok
	if p.LongContextThreshold > 0 && totalInputTokens > p.LongContextThreshold {
		rate = p.LongOutputPerMTok
	}

	return decimal.NewFromInt(int64(outputTokens)).Mul(rate).Div(million)
}

// DefaultPricing contains built-in pricing for Claude models (USD per million tokens).
// Can be overridden via WithPricing() option.
var DefaultPricing = map[anthropic.Model]ModelPricing{
	anthropic.ModelClaudeOpus4_6: {
		InputPerMTok:         decimal.NewFromFloat(5),
		OutputPerMTok:        decimal.NewFromFloat(25),
		LongInputPerMTok:     decimal.NewFromFloat(10),
		LongOutputPerMTok:    decimal.NewFromFloat(37.5),
		CacheWritePerMTok:    decimal.NewFromFloat(6.25),
		CacheReadPerMTok:     decimal.NewFromFloat(0.5),
		LongContextThreshold: 200_000,
	},
	anthropic.ModelClaudeSonnet4_5: {
		InputPerMTok:         decimal.NewFromFloat(3),
		OutputPerMTok:        decimal.NewFromFloat(15),
		LongInputPerMTok:     decimal.NewFromFloat(6),
		LongOutputPerMTok:    decimal.NewFromFloat(22.5),
		CacheWritePerMTok:    decimal.NewFromFloat(3.75),
		CacheReadPerMTok:     decimal.NewFromFloat(0.3),
		LongContextThreshold: 200_000,
	},
	anthropic.ModelClaudeHaiku4_5: {
		InputPerMTok:         decimal.NewFromFloat(1),
		OutputPerMTok:        decimal.NewFromFloat(5),
		CacheWritePerMTok:    decimal.NewFromFloat(1.25),
		CacheReadPerMTok:     decimal.NewFromFloat(0.1),
		LongContextThreshold: 0, // Haiku does not support 1M context
	},
}
