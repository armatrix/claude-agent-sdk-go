package subagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// --- Mock RunFunc helpers ---

// successRunFunc returns a RunFunc that immediately returns a successful Result.
func successRunFunc(output string) RunFunc {
	return func(ctx context.Context, childAgent *agent.Agent, prompt string) *Result {
		return &Result{
			Output: output,
			Usage:  agent.Usage{InputTokens: 100, OutputTokens: 50},
			Cost:   decimal.NewFromFloat(0.01),
		}
	}
}

// echoRunFunc returns a RunFunc that echoes the prompt in the output.
func echoRunFunc() RunFunc {
	return func(ctx context.Context, childAgent *agent.Agent, prompt string) *Result {
		return &Result{
			Output: "result for: " + prompt,
			Usage:  agent.Usage{InputTokens: 100, OutputTokens: 50},
		}
	}
}

// errorRunFunc returns a RunFunc that returns a Result with an error.
func errorRunFunc(errMsg string) RunFunc {
	return func(ctx context.Context, childAgent *agent.Agent, prompt string) *Result {
		return &Result{
			Output: errMsg,
			Err:    errors.New(errMsg),
		}
	}
}

// slowRunFunc returns a RunFunc that blocks until the context is cancelled.
func slowRunFunc() RunFunc {
	return func(ctx context.Context, childAgent *agent.Agent, prompt string) *Result {
		<-ctx.Done()
		return &Result{
			Output: "cancelled",
			Err:    ctx.Err(),
		}
	}
}

// --- Tests for buildChildOptions ---

func TestBuildChildOptions_InheritParentModel(t *testing.T) {
	parent := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeSonnet4_5))
	def := &Definition{Name: "child"}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, anthropic.ModelClaudeSonnet4_5, child.Model())
}

func TestBuildChildOptions_OverrideModel(t *testing.T) {
	parent := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeSonnet4_5))
	def := &Definition{
		Name:  "child",
		Model: anthropic.ModelClaudeHaiku4_5,
	}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, child.Model())
}

func TestBuildChildOptions_WithInstructions(t *testing.T) {
	parent := agent.NewAgent()
	def := &Definition{
		Name:         "child",
		Instructions: "You are a test agent.",
	}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, "You are a test agent.", child.Options().SystemPromptText())
}

func TestBuildChildOptions_WithMaxTurns(t *testing.T) {
	parent := agent.NewAgent()
	def := &Definition{
		Name:     "child",
		MaxTurns: 5,
	}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, 5, child.Options().MaxTurnsValue())
}

func TestBuildChildOptions_WithBudget(t *testing.T) {
	parent := agent.NewAgent()
	budget := decimal.NewFromFloat(2.50)
	def := &Definition{
		Name:      "child",
		MaxBudget: budget,
	}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)
	require.NotNil(t, child)
}

func TestBuildChildOptions_WithAdditionalOptions(t *testing.T) {
	parent := agent.NewAgent()
	def := &Definition{
		Name: "child",
		Options: []agent.AgentOption{
			agent.WithMaxTurns(10),
			agent.WithSystemPrompt("custom prompt"),
		},
	}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, 10, child.Options().MaxTurnsValue())
	assert.Equal(t, "custom prompt", child.Options().SystemPromptText())
}

func TestBuildChildOptions_AllOverrides(t *testing.T) {
	parent := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeSonnet4_5))
	def := &Definition{
		Name:         "full-override",
		Model:        anthropic.ModelClaudeHaiku4_5,
		Instructions: "Be concise.",
		MaxTurns:     3,
		MaxBudget:    decimal.NewFromFloat(1.0),
	}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, child.Model())
	assert.Equal(t, "Be concise.", child.Options().SystemPromptText())
	assert.Equal(t, 3, child.Options().MaxTurnsValue())
}

func TestBuildChildOptions_NoOverrides_InheritsParentModel(t *testing.T) {
	parent := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeOpus4_6))
	def := &Definition{Name: "minimal"}

	opts := buildChildOptions(parent, def)
	child := agent.NewAgent(opts...)

	assert.Equal(t, anthropic.ModelClaudeOpus4_6, child.Model())
}

// --- Tests for NewRunner ---

func TestNewRunner(t *testing.T) {
	parent := agent.NewAgent()
	defs := map[string]*Definition{
		"worker": {Name: "worker"},
	}

	runner := NewRunner(parent, defs)

	require.NotNil(t, runner)
	assert.Equal(t, parent, runner.parent)
	assert.Len(t, runner.defs, 1)
	assert.Equal(t, 0, runner.Active())
	assert.NotNil(t, runner.runFunc)
}

func TestNewRunnerWithRunFunc(t *testing.T) {
	parent := agent.NewAgent()
	fn := successRunFunc("ok")
	defs := map[string]*Definition{}

	runner := NewRunnerWithRunFunc(parent, defs, fn)
	require.NotNil(t, runner)
	assert.NotNil(t, runner.runFunc)
}

// --- Tests for Definitions ---

func TestDefinitions(t *testing.T) {
	defs := map[string]*Definition{
		"a": {Name: "a"},
		"b": {Name: "b"},
	}
	runner := NewRunner(agent.NewAgent(), defs)

	got := runner.Definitions()
	assert.Len(t, got, 2)
	assert.Contains(t, got, "a")
	assert.Contains(t, got, "b")
}

// --- Tests for Spawn ---

func TestSpawn_DefinitionNotFound(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{},
		successRunFunc("unused"),
	)

	_, err := runner.Spawn(context.Background(), "nonexistent", "hello")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrDefinitionNotFound)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestSpawn_ReturnsRunID(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		successRunFunc("done"),
	)

	runID, err := runner.Spawn(context.Background(), "worker", "do something")
	require.NoError(t, err)
	assert.Contains(t, runID, "run_")
}

func TestSpawn_TracksActiveRun(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		slowRunFunc(),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runID, err := runner.Spawn(ctx, "worker", "slow task")
	require.NoError(t, err)

	// Give the goroutine a moment to start.
	time.Sleep(10 * time.Millisecond)

	runner.mu.RLock()
	_, exists := runner.active[runID]
	runner.mu.RUnlock()
	assert.True(t, exists, "run should be tracked in active map")

	cancel()
}

// --- Tests for Wait ---

func TestWait_RunNotFound(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{},
		successRunFunc("unused"),
	)

	_, err := runner.Wait(context.Background(), "run_nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunNotFound)
}

func TestSpawnAndWait_Success(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		successRunFunc("task completed"),
	)

	runID, err := runner.Spawn(context.Background(), "worker", "do the work")
	require.NoError(t, err)

	result, err := runner.Wait(context.Background(), runID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "task completed", result.Output)
	assert.Equal(t, int64(100), result.Usage.InputTokens)
	assert.Equal(t, int64(50), result.Usage.OutputTokens)
	assert.True(t, result.Cost.Equal(decimal.NewFromFloat(0.01)))
	assert.NoError(t, result.Err)

	// Run should be removed from active after Wait.
	assert.Equal(t, 0, runner.Active())
}

func TestSpawnAndWait_Error(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		errorRunFunc("something went wrong"),
	)

	runID, err := runner.Spawn(context.Background(), "worker", "do the work")
	require.NoError(t, err)

	result, err := runner.Wait(context.Background(), runID)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "something went wrong")
}

func TestWait_ContextCancelled(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		slowRunFunc(),
	)

	runID, err := runner.Spawn(context.Background(), "worker", "slow task")
	require.NoError(t, err)

	// Cancel the wait context quickly.
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = runner.Wait(waitCtx, runID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunCancelled)
}

// --- Tests for Cancel ---

func TestCancel_ActiveRun(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		slowRunFunc(),
	)

	runID, err := runner.Spawn(context.Background(), "worker", "slow task")
	require.NoError(t, err)

	// Give the goroutine time to start.
	time.Sleep(10 * time.Millisecond)

	// Cancel should not panic.
	runner.Cancel(runID)

	// Wait should eventually complete since we cancelled the context.
	result, err := runner.Wait(context.Background(), runID)
	require.NoError(t, err)
	require.NotNil(t, result)
	// The slow run func returns ctx.Err() when cancelled.
	assert.Error(t, result.Err)
}

func TestCancel_NonexistentRun(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{},
		successRunFunc("unused"),
	)

	// Should not panic.
	runner.Cancel("run_nonexistent")
}

// --- Tests for multiple concurrent spawns ---

func TestMultipleSpawns(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		echoRunFunc(),
	)

	id1, err := runner.Spawn(context.Background(), "worker", "task 1")
	require.NoError(t, err)

	id2, err := runner.Spawn(context.Background(), "worker", "task 2")
	require.NoError(t, err)

	assert.NotEqual(t, id1, id2, "run IDs should be unique")

	r1, err := runner.Wait(context.Background(), id1)
	require.NoError(t, err)
	assert.Equal(t, "result for: task 1", r1.Output)

	r2, err := runner.Wait(context.Background(), id2)
	require.NoError(t, err)
	assert.Equal(t, "result for: task 2", r2.Output)
}

func TestMultipleSpawns_DifferentDefinitions(t *testing.T) {
	defs := map[string]*Definition{
		"fast": {Name: "fast", MaxTurns: 1},
		"deep": {Name: "deep", MaxTurns: 10},
	}

	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		defs,
		echoRunFunc(),
	)

	id1, err := runner.Spawn(context.Background(), "fast", "quick task")
	require.NoError(t, err)

	id2, err := runner.Spawn(context.Background(), "deep", "complex task")
	require.NoError(t, err)

	r1, err := runner.Wait(context.Background(), id1)
	require.NoError(t, err)
	assert.Equal(t, "result for: quick task", r1.Output)

	r2, err := runner.Wait(context.Background(), id2)
	require.NoError(t, err)
	assert.Equal(t, "result for: complex task", r2.Output)
}

// --- Tests for Active count ---

func TestActive_ReflectsRunState(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		slowRunFunc(),
	)

	assert.Equal(t, 0, runner.Active())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := runner.Spawn(ctx, "worker", "task 1")
	require.NoError(t, err)

	// Give the goroutine time to register.
	time.Sleep(10 * time.Millisecond)
	assert.GreaterOrEqual(t, runner.Active(), 1)

	cancel()
}

// --- Tests for RunFunc receiving correct child agent ---

func TestSpawn_ChildAgentReceivesDefinitionOverrides(t *testing.T) {
	var capturedModel anthropic.Model
	var capturedPrompt string

	capturingRunFunc := func(ctx context.Context, childAgent *agent.Agent, prompt string) *Result {
		capturedModel = childAgent.Model()
		capturedPrompt = prompt
		return &Result{Output: "captured"}
	}

	parent := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeSonnet4_5))
	defs := map[string]*Definition{
		"specialist": {
			Name:  "specialist",
			Model: anthropic.ModelClaudeHaiku4_5,
		},
	}

	runner := NewRunnerWithRunFunc(parent, defs, capturingRunFunc)

	runID, err := runner.Spawn(context.Background(), "specialist", "analyze this")
	require.NoError(t, err)

	result, err := runner.Wait(context.Background(), runID)
	require.NoError(t, err)
	assert.Equal(t, "captured", result.Output)
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5, capturedModel)
	assert.Equal(t, "analyze this", capturedPrompt)
}

// --- Tests for sentinel errors ---

func TestSentinelErrors(t *testing.T) {
	assert.NotNil(t, ErrDefinitionNotFound)
	assert.NotNil(t, ErrRunNotFound)
	assert.NotNil(t, ErrRunCancelled)

	assert.True(t, errors.Is(ErrDefinitionNotFound, ErrDefinitionNotFound))
	assert.True(t, errors.Is(ErrRunNotFound, ErrRunNotFound))
	assert.True(t, errors.Is(ErrRunCancelled, ErrRunCancelled))
}

// --- Test Wait removes handle after completion ---

func TestWait_RemovesHandleOnSuccess(t *testing.T) {
	runner := NewRunnerWithRunFunc(
		agent.NewAgent(),
		map[string]*Definition{"worker": {Name: "worker"}},
		successRunFunc("done"),
	)

	runID, err := runner.Spawn(context.Background(), "worker", "task")
	require.NoError(t, err)

	_, err = runner.Wait(context.Background(), runID)
	require.NoError(t, err)

	// Second wait should return ErrRunNotFound.
	_, err = runner.Wait(context.Background(), runID)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRunNotFound)
}
