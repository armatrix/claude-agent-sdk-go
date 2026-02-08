package tools

import (
	"context"
	"path/filepath"
	"testing"

	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/stretchr/testify/assert"
)

func TestCheckSandboxPath_NoSandbox(t *testing.T) {
	ctx := context.Background()
	err := checkSandboxPath(ctx, "/any/path")
	assert.NoError(t, err)
}

func TestCheckSandboxPath_AllowedDir(t *testing.T) {
	dir := t.TempDir()
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		AllowedDirs: []string{dir},
	})

	err := checkSandboxPath(ctx, filepath.Join(dir, "file.txt"))
	assert.NoError(t, err)
}

func TestCheckSandboxPath_BlockedDir(t *testing.T) {
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		AllowedDirs: []string{"/allowed"},
	})

	err := checkSandboxPath(ctx, "/not-allowed/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside sandbox")
}

func TestCheckSandboxPath_EmptyAllowedDirs(t *testing.T) {
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		AllowedDirs: []string{},
	})

	err := checkSandboxPath(ctx, "/any/path")
	assert.NoError(t, err)
}

func TestCheckSandboxCommand_NoSandbox(t *testing.T) {
	ctx := context.Background()
	err := checkSandboxCommand(ctx, "rm -rf /")
	assert.NoError(t, err)
}

func TestCheckSandboxCommand_AllowedCommand(t *testing.T) {
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		BlockedCommands: []string{"rm", "curl"},
	})

	err := checkSandboxCommand(ctx, "ls -la")
	assert.NoError(t, err)
}

func TestCheckSandboxCommand_BlockedCommand(t *testing.T) {
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		BlockedCommands: []string{"rm", "curl"},
	})

	err := checkSandboxCommand(ctx, "rm -rf /tmp/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked by sandbox")
}

func TestCheckSandboxCommand_BlockedCommandFullPath(t *testing.T) {
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		BlockedCommands: []string{"rm"},
	})

	err := checkSandboxCommand(ctx, "/bin/rm -rf /tmp/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "blocked by sandbox")
}

func TestCheckSandboxCommand_EmptyBlockedList(t *testing.T) {
	ctx := agent.WithContextSandbox(context.Background(), &agent.SandboxConfig{
		BlockedCommands: []string{},
	})

	err := checkSandboxCommand(ctx, "rm -rf /")
	assert.NoError(t, err)
}
