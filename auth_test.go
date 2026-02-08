package agent

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
)

func TestWithClientOptions_StoresOptions(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithClientOptions(option.WithAPIKey("test-key")),
	})
	assert.Len(t, opts.clientOptions, 1)
}

func TestWithClientOptions_Default_Nil(t *testing.T) {
	opts := resolveOptions(nil)
	assert.Nil(t, opts.clientOptions)
}

func TestWithClientOptions_Multiple(t *testing.T) {
	opts := resolveOptions([]AgentOption{
		WithClientOptions(
			option.WithAPIKey("test-key"),
			option.WithBaseURL("https://example.com"),
		),
	})
	assert.Len(t, opts.clientOptions, 2)
}

func TestNewAgent_WithClientOptions_DoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		a := NewAgent(WithClientOptions(option.WithAPIKey("test-key")))
		assert.NotNil(t, a)
	})
}
