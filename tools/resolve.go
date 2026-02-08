package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// resolvePath resolves a file path against the working directory from context.
// If the path is already absolute, it is returned as-is.
// If the context has no working directory, the path is returned as-is.
func resolvePath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if dir := agent.ContextWorkDir(ctx); dir != "" {
		return filepath.Join(dir, path)
	}
	return path
}

// checkSandboxPath validates that the resolved path is within the sandbox AllowedDirs.
// Returns nil if no sandbox is configured or the path is allowed.
func checkSandboxPath(ctx context.Context, resolved string) error {
	sandbox := agent.ContextSandbox(ctx)
	if sandbox == nil || len(sandbox.AllowedDirs) == 0 {
		return nil
	}
	absPath, err := filepath.Abs(resolved)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}
	for _, dir := range sandbox.AllowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir+string(filepath.Separator)) || absPath == absDir {
			return nil
		}
	}
	return fmt.Errorf("path %s is outside sandbox allowed directories", resolved)
}

// checkSandboxCommand validates that the command is not in the sandbox BlockedCommands list.
// Returns nil if no sandbox is configured or the command is allowed.
func checkSandboxCommand(ctx context.Context, command string) error {
	sandbox := agent.ContextSandbox(ctx)
	if sandbox == nil || len(sandbox.BlockedCommands) == 0 {
		return nil
	}
	// Extract the first word as the command name
	cmdName := strings.Fields(command)
	if len(cmdName) == 0 {
		return nil
	}
	base := filepath.Base(cmdName[0])
	for _, blocked := range sandbox.BlockedCommands {
		if base == blocked || cmdName[0] == blocked {
			return fmt.Errorf("command %q is blocked by sandbox policy", blocked)
		}
	}
	return nil
}

// applyExecContext sets cmd.Dir and cmd.Env from the agent context values.
func applyExecContext(ctx context.Context, cmd *exec.Cmd) {
	if dir := agent.ContextWorkDir(ctx); dir != "" {
		cmd.Dir = dir
	}
	if env := agent.ContextEnv(ctx); len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
}
