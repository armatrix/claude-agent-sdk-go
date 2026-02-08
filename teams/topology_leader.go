package teams

// LeaderTeammate implements the star topology: all communication is mediated
// through the leader. Teammates send to leader, leader dispatches to teammates.
type LeaderTeammate struct {
	LeaderName string
}

func (t *LeaderTeammate) Name() string { return "leader-teammate" }

func (t *LeaderTeammate) Route(from string, msg *Message, members []string) []string {
	// DM: explicit recipient
	if msg.To != "" {
		return []string{msg.To}
	}
	// Leader broadcasts to all teammates
	if from == t.LeaderName {
		out := make([]string, 0, len(members)-1)
		for _, m := range members {
			if m != from {
				out = append(out, m)
			}
		}
		return out
	}
	// Teammates route to leader
	return []string{t.LeaderName}
}

func (t *LeaderTeammate) NextTask(tasks []*Task, members []*Member) []TaskAssignment {
	// Leader assigns; no automatic assignment in this topology
	return nil
}

func (t *LeaderTeammate) OnMemberJoin(name string)  {}
func (t *LeaderTeammate) OnMemberLeave(name string) {}
