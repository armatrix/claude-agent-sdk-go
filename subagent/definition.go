// Package subagent provides parent-to-child agent delegation.
// A parent Agent can spawn child Agents via the Task tool.
// Each child runs in an independent goroutine with its own session.
package subagent

import (
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shopspring/decimal"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// Definition describes a named sub-agent that can be spawned by the parent.
type Definition struct {
	// Name is the unique identifier used to reference this sub-agent in the Task tool.
	Name string

	// Model overrides the parent's model. Empty means inherit parent.
	Model anthropic.Model

	// Instructions is an additional system prompt appended to the parent's.
	Instructions string

	// Tools is the set of tool names available to this sub-agent.
	// Nil means inherit all parent tools.
	Tools []string

	// Options are additional AgentOption functions applied to the child Agent.
	Options []agent.AgentOption

	// MaxTurns limits the sub-agent's loop iterations. 0 means inherit parent.
	MaxTurns int

	// MaxBudget limits the sub-agent's spend. Zero means inherit parent.
	MaxBudget decimal.Decimal
}
