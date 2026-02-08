package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

func TestResolvePath_Absolute(t *testing.T) {
	ctx := agent.WithContextWorkDir(context.Background(), "/work")
	assert.Equal(t, "/absolute/path", resolvePath(ctx, "/absolute/path"))
}

func TestResolvePath_Relative_WithWorkDir(t *testing.T) {
	ctx := agent.WithContextWorkDir(context.Background(), "/work")
	assert.Equal(t, "/work/foo/bar.txt", resolvePath(ctx, "foo/bar.txt"))
}

func TestResolvePath_Relative_NoWorkDir(t *testing.T) {
	assert.Equal(t, "foo/bar.txt", resolvePath(context.Background(), "foo/bar.txt"))
}

func TestResolvePath_EmptyPath_WithWorkDir(t *testing.T) {
	ctx := agent.WithContextWorkDir(context.Background(), "/work")
	assert.Equal(t, "/work", resolvePath(ctx, ""))
}
