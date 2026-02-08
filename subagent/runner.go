package subagent

import (
	"context"
	"sync"

	agent "github.com/armatrix/claude-agent-sdk-go"

	"github.com/shopspring/decimal"
)

// Result holds the output of a completed sub-agent run.
type Result struct {
	// Output is the final text response from the sub-agent.
	Output string

	// Session is the sub-agent's conversation history (useful for debugging).
	Session *agent.Session

	// Usage contains token counts for the sub-agent run.
	Usage agent.Usage

	// Cost is the total API cost incurred by the sub-agent.
	Cost decimal.Decimal

	// Err is non-nil if the sub-agent encountered an error.
	Err error
}

// runHandle tracks an active sub-agent execution.
type runHandle struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	result chan *Result
}

// Runner manages sub-agent lifecycle: spawn, track, and collect results.
type Runner struct {
	parent *agent.Agent
	defs   map[string]*Definition
	active map[string]*runHandle
	mu     sync.RWMutex
}

// NewRunner creates a Runner with the given sub-agent definitions.
func NewRunner(parent *agent.Agent, defs map[string]*Definition) *Runner {
	return &Runner{
		parent: parent,
		defs:   defs,
		active: make(map[string]*runHandle),
	}
}

// Spawn starts a sub-agent by name. Returns the run ID.
// The sub-agent runs in a background goroutine.
func (r *Runner) Spawn(ctx context.Context, name, prompt string) (string, error) {
	// TODO: implement in Phase 1 dev
	_ = ctx
	_ = name
	_ = prompt
	return "", nil
}

// Wait blocks until the given run completes and returns its result.
func (r *Runner) Wait(ctx context.Context, runID string) (*Result, error) {
	// TODO: implement in Phase 1 dev
	_ = ctx
	_ = runID
	return nil, nil
}

// Cancel stops a running sub-agent by run ID.
func (r *Runner) Cancel(runID string) {
	r.mu.RLock()
	h, ok := r.active[runID]
	r.mu.RUnlock()
	if ok {
		h.cancel()
	}
}

// Definitions returns the registered sub-agent definitions.
func (r *Runner) Definitions() map[string]*Definition {
	return r.defs
}
