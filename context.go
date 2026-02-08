package agent

import "context"

type contextKey int

const (
	ctxKeyWorkDir contextKey = iota
	ctxKeyEnv
	ctxKeySandbox
)

// WithContextWorkDir returns a context with the working directory set.
func WithContextWorkDir(ctx context.Context, dir string) context.Context {
	return context.WithValue(ctx, ctxKeyWorkDir, dir)
}

// ContextWorkDir returns the working directory from context, or empty string.
func ContextWorkDir(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyWorkDir).(string); ok {
		return v
	}
	return ""
}

// WithContextEnv returns a context with environment variables set.
func WithContextEnv(ctx context.Context, env map[string]string) context.Context {
	return context.WithValue(ctx, ctxKeyEnv, env)
}

// ContextEnv returns the environment variables from context, or nil.
func ContextEnv(ctx context.Context) map[string]string {
	if v, ok := ctx.Value(ctxKeyEnv).(map[string]string); ok {
		return v
	}
	return nil
}

// WithContextSandbox returns a context with the sandbox config set.
func WithContextSandbox(ctx context.Context, cfg *SandboxConfig) context.Context {
	return context.WithValue(ctx, ctxKeySandbox, cfg)
}

// ContextSandbox returns the sandbox config from context, or nil.
func ContextSandbox(ctx context.Context) *SandboxConfig {
	if v, ok := ctx.Value(ctxKeySandbox).(*SandboxConfig); ok {
		return v
	}
	return nil
}
