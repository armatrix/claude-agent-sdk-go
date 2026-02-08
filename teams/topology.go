// Package teams provides multi-agent collaboration with configurable topologies.
// Teams support Leader-Teammate, Pipeline, PeerRing, SupervisorTree, Blackboard,
// and MapReduce communication patterns.
package teams

// Topology defines how messages are routed and tasks are assigned within a team.
type Topology interface {
	// Name returns the topology identifier (e.g. "leader-teammate", "pipeline").
	Name() string

	// Route determines which members should receive a message.
	// from is the sender name, members is the full member list.
	// Returns the list of recipient names.
	Route(from string, msg *Message, members []string) []string

	// NextTask returns task assignments given available tasks and idle members.
	NextTask(tasks []*Task, members []*Member) []TaskAssignment

	// OnMemberJoin is called when a new member joins the team.
	OnMemberJoin(name string)

	// OnMemberLeave is called when a member leaves the team.
	OnMemberLeave(name string)
}

// TaskAssignment maps a task to a member.
type TaskAssignment struct {
	TaskID     string
	MemberName string
}
