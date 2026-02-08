package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithContextWorkDir(t *testing.T) {
	ctx := WithContextWorkDir(context.Background(), "/tmp/workdir")
	assert.Equal(t, "/tmp/workdir", ContextWorkDir(ctx))
}

func TestContextWorkDir_Empty(t *testing.T) {
	assert.Equal(t, "", ContextWorkDir(context.Background()))
}

func TestWithContextEnv(t *testing.T) {
	env := map[string]string{"FOO": "bar", "BAZ": "qux"}
	ctx := WithContextEnv(context.Background(), env)
	got := ContextEnv(ctx)
	assert.Equal(t, "bar", got["FOO"])
	assert.Equal(t, "qux", got["BAZ"])
}

func TestContextEnv_Nil(t *testing.T) {
	assert.Nil(t, ContextEnv(context.Background()))
}

func TestContextWorkDir_Chained(t *testing.T) {
	ctx := context.Background()
	ctx = WithContextWorkDir(ctx, "/a")
	ctx = WithContextEnv(ctx, map[string]string{"K": "V"})
	assert.Equal(t, "/a", ContextWorkDir(ctx))
	assert.Equal(t, "V", ContextEnv(ctx)["K"])
}
