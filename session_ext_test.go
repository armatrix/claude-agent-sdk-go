package agent

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSession_Clone(t *testing.T) {
	original := NewSession()
	original.Messages = append(original.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("hello")))
	original.Metadata.Model = "claude-opus-4-6"
	original.Metadata.TotalCost = decimal.NewFromFloat(0.05)
	original.Metadata.NumTurns = 3

	cloned := original.Clone()

	// Different ID
	assert.NotEqual(t, original.ID, cloned.ID)

	// Messages are copied
	require.Len(t, cloned.Messages, 1)

	// Metadata is copied
	assert.Equal(t, original.Metadata.Model, cloned.Metadata.Model)
	assert.True(t, original.Metadata.TotalCost.Equal(cloned.Metadata.TotalCost))
	assert.Equal(t, original.Metadata.NumTurns, cloned.Metadata.NumTurns)

	// Modifying clone does not affect original
	cloned.Messages = append(cloned.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("world")))
	assert.Len(t, original.Messages, 1)
	assert.Len(t, cloned.Messages, 2)
}

func TestSession_Clone_EmptyMessages(t *testing.T) {
	original := NewSession()
	original.ID = "fixed-id-for-test"
	cloned := original.Clone()

	assert.NotEqual(t, original.ID, cloned.ID)
	assert.Empty(t, cloned.Messages)
}
