package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanModeTool_Name(t *testing.T) {
	tool := &PlanModeTool{}
	assert.Equal(t, "ExitPlanMode", tool.Name())
}

func TestPlanModeTool_Description(t *testing.T) {
	tool := &PlanModeTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestPlanModeTool_Execute_Success(t *testing.T) {
	var receivedPlan string
	tool := &PlanModeTool{
		Callback: func(_ context.Context, plan string) error {
			receivedPlan = plan
			return nil
		},
	}

	result, err := tool.Execute(context.Background(), PlanModeInput{Plan: "Step 1: do X\nStep 2: do Y"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "Plan submitted for approval.")
	assert.Equal(t, "Step 1: do X\nStep 2: do Y", receivedPlan)
}

func TestPlanModeTool_Execute_NilCallback(t *testing.T) {
	tool := &PlanModeTool{}

	result, err := tool.Execute(context.Background(), PlanModeInput{Plan: "some plan"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "plan mode callback not configured")
}

func TestPlanModeTool_Execute_CallbackError(t *testing.T) {
	tool := &PlanModeTool{
		Callback: func(_ context.Context, _ string) error {
			return errors.New("plan rejected")
		},
	}

	result, err := tool.Execute(context.Background(), PlanModeInput{Plan: "bad plan"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "exit plan mode failed")
	assert.Contains(t, extractText(result), "plan rejected")
}

func TestPlanModeTool_Execute_EmptyPlan(t *testing.T) {
	var receivedPlan string
	tool := &PlanModeTool{
		Callback: func(_ context.Context, plan string) error {
			receivedPlan = plan
			return nil
		},
	}

	result, err := tool.Execute(context.Background(), PlanModeInput{})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "Plan submitted for approval.")
	assert.Equal(t, "", receivedPlan)
}
