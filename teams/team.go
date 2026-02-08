package teams

import (
	"context"
	"fmt"
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
	opts     teamOptions
	events   chan *Event
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	mu       sync.RWMutex
}

// Option configures a Team.
type Option func(*teamOptions)

type teamOptions struct {
	topology   Topology
	leadOpts   []agent.AgentOption
	memberDefs []memberDef
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
		opts:     o,
	}
}

// Start begins team execution with the given prompt directed to the leader.
// It creates the leader member, subscribes it to the MessageBus, starts its
// run loop, sends the initial prompt, and spawns any pre-configured members.
// Returns a Stream that aggregates events from all team members.
func (t *Team) Start(ctx context.Context, prompt string) *Stream {
	t.ctx, t.cancel = context.WithCancel(ctx)
	t.events = make(chan *Event, 64)

	// Determine leader name from topology (if LeaderTeammate) or default to "lead"
	leaderName := "lead"
	if lt, ok := t.topology.(*LeaderTeammate); ok {
		leaderName = lt.LeaderName
	}

	// Create the lead agent
	leadAgent := agent.NewAgent(t.opts.leadOpts...)

	// Create the lead member
	lead := NewMember(leaderName, RoleLead, leadAgent, t.bus)
	t.lead = lead

	t.mu.Lock()
	t.members[leaderName] = lead
	t.mu.Unlock()

	// Subscribe the leader to the message bus
	inboxCh := t.bus.Subscribe(leaderName, 64)
	// Wire the bus inbox channel to the member's inbox
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for msg := range inboxCh {
			select {
			case lead.inbox <- msg:
			case <-t.ctx.Done():
				return
			}
		}
	}()

	// Start the leader's initial agent run with the prompt
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		// Run the agent with the initial prompt
		stream := leadAgent.Run(t.ctx, prompt)
		for stream.Next() {
			select {
			case t.events <- &Event{MemberName: leaderName, AgentEvent: stream.Current()}:
			case <-t.ctx.Done():
				return
			}
		}
		lead.SetStatus(MemberIdle)

		// After the initial run, enter the message-listening loop
		lead.Run(t.ctx, t.events)
	}()

	// Notify topology about the leader
	t.topology.OnMemberJoin(leaderName)

	// Spawn pre-configured members
	for _, def := range t.opts.memberDefs {
		if err := t.SpawnMember(def.name, WithMemberAgentOptions(def.opts...)); err != nil {
			// Send error as an event
			t.events <- &Event{
				MemberName: leaderName,
				AgentEvent: &agent.ResultEvent{
					Subtype: "error_during_execution",
					IsError: true,
					Errors:  []string{fmt.Sprintf("failed to spawn member %q: %s", def.name, err.Error())},
				},
			}
		}
	}

	// Start a goroutine that waits for all members to finish and then closes the events channel
	go func() {
		t.wg.Wait()
		close(t.events)
	}()

	return &Stream{events: t.events}
}

// SpawnMember dynamically adds a new teammate to the running team.
// The member is subscribed to the MessageBus and starts its run loop.
func (t *Team) SpawnMember(name string, opts ...MemberOption) error {
	t.mu.Lock()
	if _, exists := t.members[name]; exists {
		t.mu.Unlock()
		return fmt.Errorf("member %q already exists", name)
	}
	t.mu.Unlock()

	var mo memberOptions
	for _, fn := range opts {
		fn(&mo)
	}

	// Create the member's agent
	memberAgent := agent.NewAgent(mo.agentOpts...)

	// Create the member
	member := NewMember(name, RoleTeammate, memberAgent, t.bus)

	// Subscribe to the message bus
	inboxCh := t.bus.Subscribe(name, 64)

	// Add to team's member map
	t.mu.Lock()
	t.members[name] = member
	t.mu.Unlock()

	// Notify topology
	t.topology.OnMemberJoin(name)

	// Start a goroutine that forwards bus messages to the member's inbox
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for msg := range inboxCh {
			select {
			case member.inbox <- msg:
			case <-t.ctx.Done():
				return
			}
		}
	}()

	// Start the member's run loop
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		member.Run(t.ctx, t.events)
	}()

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

// Lead returns the team leader member.
func (t *Team) Lead() *Member { return t.lead }

// Members returns a snapshot of all team members.
func (t *Team) Members() map[string]*Member {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make(map[string]*Member, len(t.members))
	for k, v := range t.members {
		result[k] = v
	}
	return result
}

// Bus returns the team's message bus.
func (t *Team) Bus() *MessageBus { return t.bus }

// TaskList returns the shared task list.
func (t *Team) TaskList() *SharedTaskList { return t.tasks }
