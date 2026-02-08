package teams

import (
	"sync/atomic"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// Role identifies a member's function within the team.
type Role int

const (
	// RoleLead is the team coordinator.
	RoleLead Role = iota
	// RoleTeammate is a regular team member.
	RoleTeammate
)

// MemberStatus tracks the current state of a team member.
type MemberStatus int32

const (
	// MemberIdle means the member is waiting for work.
	MemberIdle MemberStatus = iota
	// MemberWorking means the member is actively running.
	MemberWorking
	// MemberShutdown means the member has been stopped.
	MemberShutdown
)

// Member represents a single agent within a team.
type Member struct {
	id     string
	name   string
	role   Role
	agent  *agent.Agent
	client *agent.Client
	status atomic.Int32
	inbox  chan *Message
	bus    *MessageBus
}

// NewMember creates a member with the given name and role.
func NewMember(name string, role Role, a *agent.Agent, bus *MessageBus) *Member {
	return &Member{
		id:    agent.GenerateID(agent.PrefixAgent),
		name:  name,
		role:  role,
		agent: a,
		inbox: make(chan *Message, 64),
		bus:   bus,
	}
}

// ID returns the member's unique identifier.
func (m *Member) ID() string { return m.id }

// Name returns the member's display name.
func (m *Member) Name() string { return m.name }

// Role returns the member's role (Lead or Teammate).
func (m *Member) Role() Role { return m.role }

// Status returns the member's current status.
func (m *Member) Status() MemberStatus { return MemberStatus(m.status.Load()) }

// SetStatus atomically updates the member's status.
func (m *Member) SetStatus(s MemberStatus) { m.status.Store(int32(s)) }
