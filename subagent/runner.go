package subagent

import (
	"context"
	"errors"
	"fmt"
	"sync"

	agent "github.com/armatrix/claude-agent-sdk-go"

	"github.com/shopspring/decimal"
)

// Sentinel errors for the subagent package.
var (
	ErrDefinitionNotFound = errors.New("subagent: definition not found")
	ErrRunNotFound        = errors.New("subagent: run not found")
	ErrRunCancelled       = errors.New("subagent: run cancelled")
)

// RunFunc executes a child agent with the given prompt and returns a Result.
// The default implementation calls agent.Agent.Run and drains the stream.
// Tests can replace this to avoid real API calls.
type RunFunc func(ctx context.Context, childAgent *agent.Agent, prompt string) *Result

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
	parent  *agent.Agent
	defs    map[string]*Definition
	active  map[string]*runHandle
	mu      sync.RWMutex
	runFunc RunFunc
}

// NewRunner creates a Runner with the given sub-agent definitions.
func NewRunner(parent *agent.Agent, defs map[string]*Definition) *Runner {
	return &Runner{
		parent:  parent,
		defs:    defs,
		active:  make(map[string]*runHandle),
		runFunc: defaultRunFunc,
	}
}

// NewRunnerWithRunFunc creates a Runner with a custom run function (for testing).
func NewRunnerWithRunFunc(parent *agent.Agent, defs map[string]*Definition, fn RunFunc) *Runner {
	return &Runner{
		parent:  parent,
		defs:    defs,
		active:  make(map[string]*runHandle),
		runFunc: fn,
	}
}

// Spawn starts a sub-agent by name. Returns the run ID.
// The sub-agent runs in a background goroutine.
func (r *Runner) Spawn(ctx context.Context, name, prompt string) (string, error) {
	def, ok := r.defs[name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrDefinitionNotFound, name)
	}

	// Build child agent options from the definition overrides.
	childOpts := buildChildOptions(r.parent, def)
	childAgent := agent.NewAgent(childOpts...)

	runID := agent.GenerateID(agent.PrefixRun)
	childCtx, cancel := context.WithCancel(ctx)
	resultCh := make(chan *Result, 1)

	handle := &runHandle{
		id:     runID,
		ctx:    childCtx,
		cancel: cancel,
		result: resultCh,
	}

	r.mu.Lock()
	r.active[runID] = handle
	r.mu.Unlock()

	go func() {
		defer cancel()
		result := r.runFunc(childCtx, childAgent, prompt)
		resultCh <- result
	}()

	return runID, nil
}

// Wait blocks until the given run completes and returns its result.
func (r *Runner) Wait(ctx context.Context, runID string) (*Result, error) {
	r.mu.RLock()
	handle, ok := r.active[runID]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrRunNotFound, runID)
	}

	select {
	case <-ctx.Done():
		handle.cancel()
		r.removeHandle(runID)
		return nil, fmt.Errorf("%w: %s", ErrRunCancelled, ctx.Err())
	case result := <-handle.result:
		r.removeHandle(runID)
		return result, nil
	}
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

// Active returns the number of currently active sub-agent runs.
func (r *Runner) Active() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.active)
}

// Definitions returns the registered sub-agent definitions.
func (r *Runner) Definitions() map[string]*Definition {
	return r.defs
}

// removeHandle removes a completed run handle from the active map.
func (r *Runner) removeHandle(runID string) {
	r.mu.Lock()
	delete(r.active, runID)
	r.mu.Unlock()
}

// defaultRunFunc is the production implementation that calls Agent.Run
// and drains the resulting stream to collect the Result.
func defaultRunFunc(ctx context.Context, childAgent *agent.Agent, prompt string) *Result {
	stream := childAgent.Run(ctx, prompt)
	return drainStream(stream)
}

// buildChildOptions constructs AgentOption functions from a Definition,
// inheriting the parent's model when the definition does not override it.
func buildChildOptions(parent *agent.Agent, def *Definition) []agent.AgentOption {
	var opts []agent.AgentOption

	// Inherit or override model.
	if def.Model != "" {
		opts = append(opts, agent.WithModel(def.Model))
	} else {
		opts = append(opts, agent.WithModel(parent.Model()))
	}

	// Override system prompt if specified.
	if def.Instructions != "" {
		opts = append(opts, agent.WithSystemPrompt(def.Instructions))
	}

	// Override max turns if specified.
	if def.MaxTurns > 0 {
		opts = append(opts, agent.WithMaxTurns(def.MaxTurns))
	}

	// Override budget if specified.
	if !def.MaxBudget.IsZero() {
		opts = append(opts, agent.WithBudget(def.MaxBudget))
	}

	// Apply any additional options from the definition.
	opts = append(opts, def.Options...)

	return opts
}

// drainStream iterates an AgentStream to completion and collects the Result.
func drainStream(stream *agent.AgentStream) *Result {
	result := &Result{
		Session: stream.Session(),
	}

	for stream.Next() {
		evt := stream.Current()

		switch e := evt.(type) {
		case *agent.ResultEvent:
			result.Output = e.Result
			result.Usage = e.Usage
			result.Cost = e.TotalCost
			if e.IsError {
				result.Err = fmt.Errorf("subagent error: %s", e.Result)
			}
		}
	}

	if err := stream.Err(); err != nil {
		result.Err = err
	}

	return result
}
