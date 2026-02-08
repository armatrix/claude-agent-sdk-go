package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// ShutdownRequestInput defines the input for the ShutdownRequest tool.
type ShutdownRequestInput struct {
	Recipient string `json:"recipient" jsonschema:"required,description=Name of the team member to shut down"`
	Reason    string `json:"reason,omitempty" jsonschema:"description=Reason for shutdown"`
}

// ShutdownRequestTool sends a shutdown request to a team member via the MessageBus.
type ShutdownRequestTool struct {
	Bus        *teams.MessageBus
	SenderName string
}

var _ agent.Tool[ShutdownRequestInput] = (*ShutdownRequestTool)(nil)

func (t *ShutdownRequestTool) Name() string { return "ShutdownRequest" }
func (t *ShutdownRequestTool) Description() string {
	return "Request a team member to gracefully shut down"
}

func (t *ShutdownRequestTool) Execute(_ context.Context, input ShutdownRequestInput) (*agent.ToolResult, error) {
	if input.Recipient == "" {
		return agent.ErrorResult("recipient is required"), nil
	}

	content := "shutdown requested"
	if input.Reason != "" {
		content = input.Reason
	}

	msg := teams.NewMessage(teams.MessageShutdownRequest, t.SenderName, input.Recipient, content)
	msg.RequestID = agent.GenerateID("req")

	if err := t.Bus.Send(msg); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to send shutdown request: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Shutdown request sent to %s (request_id=%s)", input.Recipient, msg.RequestID)), nil
}
