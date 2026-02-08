package tools

import (
	"context"
	"fmt"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/teams"
)

// BroadcastInput defines the input for the Broadcast tool.
type BroadcastInput struct {
	Content string `json:"content" jsonschema:"required,description=Message to broadcast to all team members"`
}

// BroadcastTool broadcasts a message to all team members via the MessageBus.
type BroadcastTool struct {
	Bus        *teams.MessageBus
	SenderName string
}

var _ agent.Tool[BroadcastInput] = (*BroadcastTool)(nil)

// NewBroadcastTool creates a BroadcastTool for the given member.
func NewBroadcastTool(bus *teams.MessageBus, senderName string) *BroadcastTool {
	return &BroadcastTool{Bus: bus, SenderName: senderName}
}

func (t *BroadcastTool) Name() string { return "Broadcast" }
func (t *BroadcastTool) Description() string {
	return "Broadcast a message to all team members"
}

func (t *BroadcastTool) Execute(_ context.Context, input BroadcastInput) (*agent.ToolResult, error) {
	if input.Content == "" {
		return agent.ErrorResult("content is required"), nil
	}

	msg := teams.NewMessage(teams.MessageBroadcast, t.SenderName, "", input.Content)
	if err := t.Bus.Broadcast(msg); err != nil {
		return agent.ErrorResult(fmt.Sprintf("failed to broadcast: %s", err.Error())), nil
	}

	members := t.Bus.MemberNames()
	count := 0
	for _, name := range members {
		if name != t.SenderName {
			count++
		}
	}

	return agent.TextResult(fmt.Sprintf("Message broadcast to %d members", count)), nil
}
