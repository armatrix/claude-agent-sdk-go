package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPreset_Exists(t *testing.T) {
	content, ok := GetPreset("claude_code")
	assert.True(t, ok)
	assert.NotEmpty(t, content)
}

func TestGetPreset_NotFound(t *testing.T) {
	content, ok := GetPreset("nonexistent")
	assert.False(t, ok)
	assert.Empty(t, content)
}
