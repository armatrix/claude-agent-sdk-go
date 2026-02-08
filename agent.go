package agent

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
)

// Agent is a stateless execution engine that holds configuration, tools, and hooks.
// The same Agent can be safely shared across multiple goroutines and Clients.
type Agent struct {
	apiClient *anthropic.Client
	opts      agentOptions
}

// NewAgent creates a new Agent with the given options.
// The Agent is stateless — it does not hold any session or conversation history.
func NewAgent(opts ...AgentOption) *Agent {
	resolved := resolveOptions(opts)

	client := anthropic.NewClient()

	return &Agent{
		apiClient: &client,
		opts:      resolved,
	}
}

// Run starts a single-shot agent execution with a new session.
// Returns an AgentStream for iterating over events.
//
// Stub implementation — will be wired to the internal agent loop in Task 3.
func (a *Agent) Run(ctx context.Context, prompt string) *AgentStream {
	return a.RunWithSession(ctx, NewSession(), prompt)
}

// RunWithSession starts an agent execution using an existing session.
// The session's message history is preserved and extended.
//
// Stub implementation — will be wired to the internal agent loop in Task 3.
func (a *Agent) RunWithSession(ctx context.Context, session *Session, prompt string) *AgentStream {
	_ = ctx
	_ = prompt
	return emptyStream()
}

// Model returns the configured model name.
func (a *Agent) Model() string {
	return a.opts.model
}

// Options returns a copy of the resolved agent options (for testing/inspection).
func (a *Agent) Options() agentOptions {
	return a.opts
}
