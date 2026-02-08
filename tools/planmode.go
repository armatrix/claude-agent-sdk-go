package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// PlanCallback is called when the agent wants to exit plan mode.
type PlanCallback func(ctx context.Context, plan string) error

// PlanModeInput defines the input for the ExitPlanMode tool.
type PlanModeInput struct {
	Plan string `json:"plan,omitempty" jsonschema:"description=The finalized plan content"`
}

// PlanModeTool signals that the agent is ready to exit plan mode.
type PlanModeTool struct {
	Callback PlanCallback
}

var _ agent.Tool[PlanModeInput] = (*PlanModeTool)(nil)

func (t *PlanModeTool) Name() string        { return "ExitPlanMode" }
func (t *PlanModeTool) Description() string  { return "Signal that planning is complete and ready for user approval" }

func (t *PlanModeTool) Execute(ctx context.Context, input PlanModeInput) (*agent.ToolResult, error) {
	if t.Callback == nil {
		return agent.ErrorResult("plan mode callback not configured"), nil
	}

	if err := t.Callback(ctx, input.Plan); err != nil {
		return agent.ErrorResult(fmt.Sprintf("exit plan mode failed: %s", err.Error())), nil
	}
	return agent.TextResult("Plan submitted for approval."), nil
}
