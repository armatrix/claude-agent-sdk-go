package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/armatrix/claude-agent-sdk-go/teams"
)

func TestSendMessageTool_Name(t *testing.T) {
	tool := &SendMessageTool{}
	assert.Equal(t, "SendMessage", tool.Name())
}

func TestSendMessageTool_Description(t *testing.T) {
	tool := &SendMessageTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestSendMessageTool_Execute_Success(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})
	chBob := bus.Subscribe("bob", 10)
	bus.Subscribe("alice", 10)

	tool := &SendMessageTool{
		Bus:        bus,
		SenderName: "alice",
	}

	result, err := tool.Execute(context.Background(), SendMessageInput{
		Recipient: "bob",
		Content:   "hello bob",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "Message sent to bob")

	// Verify message was delivered
	select {
	case msg := <-chBob:
		assert.Equal(t, "hello bob", msg.Content)
		assert.Equal(t, "alice", msg.From)
		assert.Equal(t, "bob", msg.To)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}
}

func TestSendMessageTool_Execute_EmptyRecipient(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})

	tool := &SendMessageTool{
		Bus:        bus,
		SenderName: "alice",
	}

	result, err := tool.Execute(context.Background(), SendMessageInput{
		Recipient: "",
		Content:   "hello",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "recipient is required")
}

func TestSendMessageTool_Execute_EmptyContent(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})

	tool := &SendMessageTool{
		Bus:        bus,
		SenderName: "alice",
	}

	result, err := tool.Execute(context.Background(), SendMessageInput{
		Recipient: "bob",
		Content:   "",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "content is required")
}

func TestSendMessageTool_Execute_RecipientNotFound(t *testing.T) {
	bus := teams.NewMessageBus(&teams.LeaderTeammate{LeaderName: "lead"})
	bus.Subscribe("alice", 10)

	tool := &SendMessageTool{
		Bus:        bus,
		SenderName: "alice",
	}

	result, err := tool.Execute(context.Background(), SendMessageInput{
		Recipient: "nobody",
		Content:   "hello?",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "not found")
}
