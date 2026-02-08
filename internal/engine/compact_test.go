package engine

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToBetaParams_Basic(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		},
	}

	compact := CompactConfig{
		Strategy:      CompactServer,
		TriggerTokens: 100000,
	}

	beta := convertToBetaParams(params, compact, nil)

	assert.Equal(t, anthropic.ModelClaudeOpus4_6, beta.Model)
	assert.Equal(t, int64(4096), beta.MaxTokens)
	require.Len(t, beta.Messages, 1)
	assert.Equal(t, anthropic.BetaMessageParamRole("user"), beta.Messages[0].Role)

	// Check context management
	require.Len(t, beta.ContextManagement.Edits, 1)
	edit := beta.ContextManagement.Edits[0]
	require.NotNil(t, edit.OfCompact20260112)
	assert.Equal(t, int64(100000), edit.OfCompact20260112.Trigger.Value)

	// Check beta header
	require.Len(t, beta.Betas, 1)
	assert.Equal(t, anthropic.AnthropicBeta("compact-2026-01-12"), beta.Betas[0])
}

func TestConvertToBetaParams_WithPauseAndInstructions(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		},
	}

	compact := CompactConfig{
		Strategy:          CompactServer,
		TriggerTokens:     80000,
		PauseAfterCompact: true,
		Instructions:      "Preserve file paths and variable names",
	}

	beta := convertToBetaParams(params, compact, nil)

	edit := beta.ContextManagement.Edits[0].OfCompact20260112
	require.NotNil(t, edit)
	assert.Equal(t, int64(80000), edit.Trigger.Value)
}

func TestConvertToBetaParams_WithTools(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Use tools")),
		},
		Tools: []anthropic.ToolUnionParam{
			{
				OfTool: &anthropic.ToolParam{
					Name: "read_file",
				},
			},
		},
	}

	compact := CompactConfig{
		Strategy:      CompactServer,
		TriggerTokens: 100000,
	}

	beta := convertToBetaParams(params, compact, nil)

	require.Len(t, beta.Tools, 1)
	require.NotNil(t, beta.Tools[0].OfTool)
	assert.Equal(t, "read_file", beta.Tools[0].OfTool.Name)
}

func TestConvertMessageParam_Text(t *testing.T) {
	msg := anthropic.NewUserMessage(anthropic.NewTextBlock("Hello world"))
	beta := convertMessageParam(msg)

	assert.Equal(t, anthropic.BetaMessageParamRole("user"), beta.Role)
	require.Len(t, beta.Content, 1)
	require.NotNil(t, beta.Content[0].OfText)
	assert.Equal(t, "Hello world", beta.Content[0].OfText.Text)
}

func TestConvertMessageParam_ToolResult(t *testing.T) {
	msg := anthropic.NewUserMessage(
		anthropic.NewToolResultBlock("toolu_123", "file contents here", false),
	)
	beta := convertMessageParam(msg)

	assert.Equal(t, anthropic.BetaMessageParamRole("user"), beta.Role)
	require.Len(t, beta.Content, 1)
	require.NotNil(t, beta.Content[0].OfToolResult)
	assert.Equal(t, "toolu_123", beta.Content[0].OfToolResult.ToolUseID)
}

func TestConvertMessageParam_Assistant(t *testing.T) {
	msg := anthropic.NewAssistantMessage(anthropic.NewTextBlock("I'll help you"))
	beta := convertMessageParam(msg)

	assert.Equal(t, anthropic.BetaMessageParamRole("assistant"), beta.Role)
	require.Len(t, beta.Content, 1)
	require.NotNil(t, beta.Content[0].OfText)
	assert.Equal(t, "I'll help you", beta.Content[0].OfText.Text)
}

func TestConvertContentBlockParam_ToolUse(t *testing.T) {
	block := anthropic.NewToolUseBlock("toolu_456", map[string]string{"key": "val"}, "my_tool")
	beta := convertContentBlockParam(block)

	require.NotNil(t, beta.OfToolUse)
	assert.Equal(t, "toolu_456", beta.OfToolUse.ID)
	assert.Equal(t, "my_tool", beta.OfToolUse.Name)
}

// --- Beta merging ---

func TestConvertToBetaParams_MergesUserBetas(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		},
	}

	compact := CompactConfig{
		Strategy:      CompactServer,
		TriggerTokens: 100000,
	}

	beta := convertToBetaParams(params, compact, []string{"context-1m-2025-08-07"})

	// Should have both compact and user beta
	require.Len(t, beta.Betas, 2)
	assert.Equal(t, anthropic.AnthropicBeta("compact-2026-01-12"), beta.Betas[0])
	assert.Equal(t, anthropic.AnthropicBeta("context-1m-2025-08-07"), beta.Betas[1])
}

func TestConvertToBetaParams_DeduplicatesBetas(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		},
	}

	compact := CompactConfig{
		Strategy:      CompactServer,
		TriggerTokens: 100000,
	}

	// User passes the same beta as compact
	beta := convertToBetaParams(params, compact, []string{"compact-2026-01-12"})

	// Should deduplicate
	require.Len(t, beta.Betas, 1)
	assert.Equal(t, anthropic.AnthropicBeta("compact-2026-01-12"), beta.Betas[0])
}

func TestConvertToBetaParams_PropagatesThinking(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 128000,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Think")),
		},
		Thinking: anthropic.ThinkingConfigParamOfEnabled(10000),
	}

	compact := CompactConfig{
		Strategy:      CompactServer,
		TriggerTokens: 100000,
	}

	beta := convertToBetaParams(params, compact, nil)

	// Thinking should be propagated
	require.NotNil(t, beta.Thinking.OfEnabled)
	assert.Equal(t, int64(10000), beta.Thinking.OfEnabled.BudgetTokens)
}

// --- convertToBetaParamsNoCompact ---

func TestConvertToBetaParamsNoCompact_Basic(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 4096,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
		},
	}

	beta := convertToBetaParamsNoCompact(params, []string{"context-1m-2025-08-07"})

	assert.Equal(t, anthropic.ModelClaudeOpus4_6, beta.Model)
	assert.Equal(t, int64(4096), beta.MaxTokens)
	require.Len(t, beta.Messages, 1)
	require.Len(t, beta.Betas, 1)
	assert.Equal(t, anthropic.AnthropicBeta("context-1m-2025-08-07"), beta.Betas[0])
}

func TestConvertToBetaParamsNoCompact_WithThinking(t *testing.T) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeOpus4_6,
		MaxTokens: 128000,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Think")),
		},
		Thinking: anthropic.ThinkingConfigParamOfEnabled(50000),
	}

	beta := convertToBetaParamsNoCompact(params, []string{"my-beta"})

	require.NotNil(t, beta.Thinking.OfEnabled)
	assert.Equal(t, int64(50000), beta.Thinking.OfEnabled.BudgetTokens)
}

// --- mergeBetas ---

func TestMergeBetas_Empty(t *testing.T) {
	result := mergeBetas(nil, nil)
	assert.Len(t, result, 0)
}

func TestMergeBetas_InternalOnly(t *testing.T) {
	result := mergeBetas([]string{"a", "b"}, nil)
	require.Len(t, result, 2)
	assert.Equal(t, anthropic.AnthropicBeta("a"), result[0])
	assert.Equal(t, anthropic.AnthropicBeta("b"), result[1])
}

func TestMergeBetas_UserOnly(t *testing.T) {
	result := mergeBetas(nil, []string{"x", "y"})
	require.Len(t, result, 2)
	assert.Equal(t, anthropic.AnthropicBeta("x"), result[0])
	assert.Equal(t, anthropic.AnthropicBeta("y"), result[1])
}

func TestMergeBetas_Dedup(t *testing.T) {
	result := mergeBetas([]string{"a", "b"}, []string{"b", "c"})
	require.Len(t, result, 3)
	assert.Equal(t, anthropic.AnthropicBeta("a"), result[0])
	assert.Equal(t, anthropic.AnthropicBeta("b"), result[1])
	assert.Equal(t, anthropic.AnthropicBeta("c"), result[2])
}
