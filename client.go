package agent

import (
	"context"
)

// Client is a stateful session container that wraps an Agent.
// It maintains conversation history across multiple Query calls.
type Client struct {
	agent   *Agent
	session *Session
	store   SessionStore
}

// NewClient creates a new Client with its own Agent configured by the given options.
func NewClient(opts ...AgentOption) *Client {
	return &Client{
		agent:   NewAgent(opts...),
		session: NewSession(),
	}
}

// Query sends a prompt to the agent within the client's ongoing session.
// The session history is automatically maintained across calls.
//
// Stub implementation — will be wired to the internal agent loop in Task 3.
func (c *Client) Query(ctx context.Context, prompt string) *AgentStream {
	return c.agent.RunWithSession(ctx, c.session, prompt)
}

// Session returns the client's current session.
func (c *Client) Session() *Session {
	return c.session
}

// Agent returns the underlying Agent.
func (c *Client) Agent() *Agent {
	return c.agent
}

// Close persists the session (if a store is configured) and releases resources.
func (c *Client) Close() error {
	if c.store != nil {
		return c.store.Save(context.Background(), c.session)
	}
	return nil
}
