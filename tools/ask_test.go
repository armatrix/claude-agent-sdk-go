package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAskTool_Name(t *testing.T) {
	tool := &AskTool{}
	assert.Equal(t, "AskUserQuestion", tool.Name())
}

func TestAskTool_Description(t *testing.T) {
	tool := &AskTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestAskTool_Execute_Success(t *testing.T) {
	tool := &AskTool{
		Callback: func(_ context.Context, question string, _ []AskOption) (string, error) {
			assert.Equal(t, "What color?", question)
			return "blue", nil
		},
	}

	result, err := tool.Execute(context.Background(), AskInput{Question: "What color?"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "blue", extractText(result))
}

func TestAskTool_Execute_EmptyQuestion(t *testing.T) {
	tool := &AskTool{
		Callback: func(_ context.Context, _ string, _ []AskOption) (string, error) {
			t.Fatal("callback should not be called")
			return "", nil
		},
	}

	result, err := tool.Execute(context.Background(), AskInput{Question: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "question is required")
}

func TestAskTool_Execute_NilCallback(t *testing.T) {
	tool := &AskTool{}

	result, err := tool.Execute(context.Background(), AskInput{Question: "hello?"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "ask callback not configured")
}

func TestAskTool_Execute_WithOptions(t *testing.T) {
	opts := []AskOption{
		{Label: "Option A", Description: "First option"},
		{Label: "Option B", Description: "Second option"},
	}
	optsJSON, err := json.Marshal(opts)
	require.NoError(t, err)

	var receivedOptions []AskOption
	tool := &AskTool{
		Callback: func(_ context.Context, _ string, options []AskOption) (string, error) {
			receivedOptions = options
			return "Option A", nil
		},
	}

	result, err := tool.Execute(context.Background(), AskInput{
		Question: "Pick one",
		Options:  optsJSON,
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "Option A", extractText(result))
	require.Len(t, receivedOptions, 2)
	assert.Equal(t, "Option A", receivedOptions[0].Label)
	assert.Equal(t, "First option", receivedOptions[0].Description)
	assert.Equal(t, "Option B", receivedOptions[1].Label)
}

func TestAskTool_Execute_InvalidOptionsJSON(t *testing.T) {
	var receivedOptions []AskOption
	tool := &AskTool{
		Callback: func(_ context.Context, _ string, options []AskOption) (string, error) {
			receivedOptions = options
			return "ok", nil
		},
	}

	result, err := tool.Execute(context.Background(), AskInput{
		Question: "Pick one",
		Options:  json.RawMessage(`not valid json`),
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Equal(t, "ok", extractText(result))
	assert.Nil(t, receivedOptions)
}

func TestAskTool_Execute_CallbackError(t *testing.T) {
	tool := &AskTool{
		Callback: func(_ context.Context, _ string, _ []AskOption) (string, error) {
			return "", errors.New("user cancelled")
		},
	}

	result, err := tool.Execute(context.Background(), AskInput{Question: "hello?"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "ask failed")
	assert.Contains(t, extractText(result), "user cancelled")
}

func TestAskTool_Execute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tool := &AskTool{
		Callback: func(ctx context.Context, _ string, _ []AskOption) (string, error) {
			return "", ctx.Err()
		},
	}

	result, err := tool.Execute(ctx, AskInput{Question: "hello?"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "ask failed")
}
