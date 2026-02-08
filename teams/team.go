package teams

import (
	"context"
	"sync"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// Team is the top-level container for multi-agent collaboration.
type Team struct {
	id       string
	name     string
	lead     *Member
	members  map[string]*Member
	tasks    *SharedTaskList
	bus      *MessageBus
	topology Topology
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.RWMutex
}

// Option configures a Team.
type Option func(*teamOptions)

type teamOptions struct {
	topology     Topology
	leadOpts     []agent.AgentOption
	memberDefs   []memberDef
}

type memberDef struct {
	name string
	opts []agent.AgentOption
}

// WithTopology sets the team's communication topology.
func WithTopology(t Topology) Option {
	return func(o *teamOptions) { o.topology = t }
}

// WithLeadAgent sets options for the team leader's agent.
func WithLeadAgent(opts ...agent.AgentOption) Option {
	return func(o *teamOptions) { o.leadOpts = opts }
}

// WithMember adds a named teammate with the given agent options.
func WithMember(name string, opts ...agent.AgentOption) Option {
	return func(o *teamOptions) {
		o.memberDefs = append(o.memberDefs, memberDef{name: name, opts: opts})
	}
}

// MemberOption configures a dynamically spawned member.
type MemberOption func(*memberOptions)

type memberOptions struct {
	agentOpts []agent.AgentOption
}

// WithMemberAgentOptions sets agent options for a spawned member.
func WithMemberAgentOptions(opts ...agent.AgentOption) MemberOption {
	return func(o *memberOptions) { o.agentOpts = opts }
}

// New creates a Team with the given name and options.
func New(name string, opts ...Option) *Team {
	var o teamOptions
	for _, fn := range opts {
		fn(&o)
	}
	if o.topology == nil {
		o.topology = &LeaderTeammate{LeaderName: "lead"}
	}

	return &Team{
		id:       agent.GenerateID(agent.PrefixTeam),
		name:     name,
		members:  make(map[string]*Member),
		tasks:    NewSharedTaskList(),
		bus:      NewMessageBus(o.topology),
		topology: o.topology,
	}
}

// Start begins team execution with the given prompt directed to the leader.
func (t *Team) Start(ctx context.Context, prompt string) *Stream {
	// TODO: implement — create leader, start loop, return stream
	t.ctx, t.cancel = context.WithCancel(ctx)
	_ = prompt
	return &Stream{events: make(chan *Event, 64)}
}

// SpawnMember dynamically adds a new teammate to the running team.
func (t *Team) SpawnMember(name string, opts ...MemberOption) error {
	// TODO: implement
	_ = name
	_ = opts
	return nil
}

// Shutdown gracefully stops all team members.
func (t *Team) Shutdown() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// ID returns the team's unique identifier.
func (t *Team) ID() string { return t.id }

// Name returns the team's name.
func (t *Team) Name() string { return t.name }

// TaskList returns the shared task list.
func (t *Team) TaskList() *SharedTaskList { return t.tasks }
