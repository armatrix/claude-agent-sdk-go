package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// SendMessageInput defines the input for the SendMessage tool.
type SendMessageInput struct {
	Recipient string `json:"recipient" jsonschema:"required,description=Name of the team member to message"`
	Content   string `json:"content" jsonschema:"required,description=Message content"`
}

// SendMessageTool sends a direct message to another team member via the MessageBus.
type SendMessageTool struct {
	Bus        *teams.MessageBus
	SenderName string
}

var _ agent.Tool[SendMessageInput] = (*SendMessageTool)(nil)

// NewSendMessageTool creates a SendMessageTool for the given member.
func NewSendMessageTool(bus *teams.MessageBus, senderName string) *SendMessageTool {
	return &SendMessageTool{Bus: bus, SenderName: senderName}
}

func (t *SendMessageTool) Name() string { return "SendMessage" }
func (t *SendMessageTool) Description() string {
	return "Send a direct message to a specific team member"
}

func (t *SendMessageTool) Execute(_ context.Context, input SendMessageInput) (*agent.ToolResult, error) {
	if input.Recipient == "" {
		return agent.ErrorResult("recipient is required"), nil
	}
	if input.Content == "" {
		return agent.ErrorResult("content is required"), nil
	}

	msg := teams.NewMessage(teams.MessageDM, t.SenderName, input.Recipient, input.Content)
	if err := t.Bus.Send(msg); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to send message: %s", err.Error())), nil
	}

	return agent.TextResult(fmt.Sprintf("Message sent to %s", input.Recipient)), nil
}
