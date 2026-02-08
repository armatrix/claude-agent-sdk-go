package agent

import "errors"

// Sentinel errors returned by the agent loop and client operations.
var (
	ErrBudgetExhausted = errors.New("agent: budget exhausted")
	ErrMaxTurns        = errors.New("agent: max turns reached")
	ErrContextOverflow = errors.New("agent: context window overflow")
	ErrNoSessionStore  = errors.New("agent: no session store configured")
)
