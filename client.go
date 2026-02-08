package agent

import (
	"context"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"

	"github.com/armatrix/claude-agent-sdk-go/permission"
)

// Client is a stateful session container that wraps an Agent.
// It maintains conversation history across multiple Query calls.
type Client struct {
	agent   *Agent
	session *Session
	store   SessionStore

	mu     sync.Mutex
	cancel context.CancelFunc // cancel for current Query
}

// NewClient creates a new Client with its own Agent configured by the given options.
func NewClient(opts ...AgentOption) *Client {
	resolved := resolveOptions(opts)
	c := &Client{
		agent:   NewAgent(opts...),
		session: NewSession(),
	}
	if resolved.sessionStore != nil {
		c.store = resolved.sessionStore
	}
	return c
}

// Query sends a prompt to the agent within the client's ongoing session.
// The session history is automatically maintained across calls.
func (c *Client) Query(ctx context.Context, prompt string) *AgentStream {
	c.mu.Lock()
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()

	return c.agent.RunWithSession(ctx, c.session, prompt)
}

// Interrupt cancels the currently running Query, if any.
func (c *Client) Interrupt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// Resume loads a session from the store and replaces the current session.
// Requires a SessionStore to be configured via WithSessionStore.
func (c *Client) Resume(ctx context.Context, sessionID string) error {
	if c.store == nil {
		return errNoStore
	}
	session, err := c.store.Load(ctx, sessionID)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.session = session
	c.mu.Unlock()
	return nil
}

// Fork creates a new Client that shares the same Agent but has a cloned session.
func (c *Client) Fork() *Client {
	c.mu.Lock()
	cloned := c.session.Clone()
	c.mu.Unlock()

	return &Client{
		agent:   c.agent,
		session: cloned,
		store:   c.store,
	}
}

// SetModel updates the agent's model for subsequent queries.
func (c *Client) SetModel(model anthropic.Model) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agent.opts.model = model
}

// SetMaxThinkingTokens updates the thinking token budget for subsequent queries.
// Set to 0 to disable thinking.
func (c *Client) SetMaxThinkingTokens(n int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agent.opts.maxThinkingTokens = n
}

// InterruptAndContinue cancels the current Query but preserves the session.
// The next call to Query will continue the conversation from where it left off.
// This is semantically equivalent to Interrupt — included for API parity with
// the official TS/Python SDKs.
func (c *Client) InterruptAndContinue() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

// SetPermissionMode updates the permission mode for subsequent queries.
func (c *Client) SetPermissionMode(mode permission.Mode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.agent.opts.permissionMode = mode
}

// ContinueLatest loads the most recently updated session from the store.
// Requires a SessionStore that implements SessionLister.
func (c *Client) ContinueLatest(ctx context.Context) error {
	if c.store == nil {
		return errNoStore
	}
	lister, ok := c.store.(SessionLister)
	if !ok {
		return ErrStoreNotListable
	}
	sessions, err := lister.List(ctx)
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return ErrNoSessions
	}
	// Find most recently updated
	latest := sessions[0]
	for _, s := range sessions[1:] {
		if s.UpdatedAt.After(latest.UpdatedAt) {
			latest = s
		}
	}
	c.mu.Lock()
	c.session = latest
	c.mu.Unlock()
	return nil
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

var errNoStore = ErrNoSessionStore
