package teams

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeaderTeammate_Name(t *testing.T) {
	topo := &LeaderTeammate{LeaderName: "lead"}
	assert.Equal(t, "leader-teammate", topo.Name())
}

func TestLeaderTeammate_Route_TeammateToLeader(t *testing.T) {
	topo := &LeaderTeammate{LeaderName: "lead"}
	members := []string{"lead", "alice", "bob"}

	msg := &Message{From: "alice"}
	targets := topo.Route("alice", msg, members)
	assert.Equal(t, []string{"lead"}, targets)
}

func TestLeaderTeammate_Route_LeaderBroadcasts(t *testing.T) {
	topo := &LeaderTeammate{LeaderName: "lead"}
	members := []string{"lead", "alice", "bob"}

	msg := &Message{From: "lead"}
	targets := topo.Route("lead", msg, members)
	assert.Len(t, targets, 2)
	assert.Contains(t, targets, "alice")
	assert.Contains(t, targets, "bob")
	assert.NotContains(t, targets, "lead")
}

func TestLeaderTeammate_Route_ExplicitRecipient(t *testing.T) {
	topo := &LeaderTeammate{LeaderName: "lead"}
	members := []string{"lead", "alice", "bob"}

	msg := &Message{From: "lead", To: "alice"}
	targets := topo.Route("lead", msg, members)
	assert.Equal(t, []string{"alice"}, targets)
}

func TestPipeline_Name(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b"}}
	assert.Equal(t, "pipeline", topo.Name())
}

func TestPipeline_Route_NextStage(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b", "c"}}
	members := []string{"a", "b", "c"}

	msg := &Message{From: "a"}
	targets := topo.Route("a", msg, members)
	assert.Equal(t, []string{"b"}, targets)

	msg2 := &Message{From: "b"}
	targets2 := topo.Route("b", msg2, members)
	assert.Equal(t, []string{"c"}, targets2)
}

func TestPipeline_Route_LastStage(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b"}}
	members := []string{"a", "b"}

	msg := &Message{From: "b"}
	targets := topo.Route("b", msg, members)
	assert.Nil(t, targets)
}

func TestPipeline_Route_ExplicitRecipient(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b", "c"}}
	members := []string{"a", "b", "c"}

	msg := &Message{From: "a", To: "c"}
	targets := topo.Route("a", msg, members)
	assert.Equal(t, []string{"c"}, targets)
}

func TestPeerRing_Name(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	assert.Equal(t, "peer-ring", topo.Name())
}

func TestPeerRing_Route_NextInRing(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	members := []string{"a", "b", "c"}

	msg := &Message{From: "a"}
	targets := topo.Route("a", msg, members)
	assert.Equal(t, []string{"b"}, targets)

	msg2 := &Message{From: "c"}
	targets2 := topo.Route("c", msg2, members)
	assert.Equal(t, []string{"a"}, targets2) // wraps around
}

func TestPeerRing_Route_ExplicitRecipient(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	members := []string{"a", "b", "c"}

	msg := &Message{From: "a", To: "c"}
	targets := topo.Route("a", msg, members)
	assert.Equal(t, []string{"c"}, targets)
}

func TestSupervisorTree_Name(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{},
		Children: map[string][]string{},
	}
	assert.Equal(t, "supervisor-tree", topo.Name())
}

func TestSupervisorTree_Route_ChildToParent(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"child1": "root", "child2": "root"},
		Children: map[string][]string{"root": {"child1", "child2"}},
	}
	members := []string{"root", "child1", "child2"}

	msg := &Message{From: "child1"}
	targets := topo.Route("child1", msg, members)
	assert.Equal(t, []string{"root"}, targets)
}

func TestSupervisorTree_Route_RootNoParent(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"child": "root"},
		Children: map[string][]string{"root": {"child"}},
	}
	members := []string{"root", "child"}

	msg := &Message{From: "root"}
	targets := topo.Route("root", msg, members)
	assert.Nil(t, targets)
}

func TestSupervisorTree_Route_ExplicitRecipient(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"child": "root"},
		Children: map[string][]string{"root": {"child"}},
	}
	members := []string{"root", "child"}

	msg := &Message{From: "root", To: "child"}
	targets := topo.Route("root", msg, members)
	assert.Equal(t, []string{"child"}, targets)
}

func TestBlackboardTopology_Name(t *testing.T) {
	topo := &BlackboardTopology{Board: NewBlackboard()}
	assert.Equal(t, "blackboard", topo.Name())
}

func TestBlackboardTopology_Route_NoDirectRouting(t *testing.T) {
	topo := &BlackboardTopology{Board: NewBlackboard()}
	members := []string{"a", "b", "c"}

	msg := &Message{From: "a"}
	targets := topo.Route("a", msg, members)
	assert.Nil(t, targets)
}

func TestBlackboardTopology_Route_ExplicitDM(t *testing.T) {
	topo := &BlackboardTopology{Board: NewBlackboard()}
	members := []string{"a", "b"}

	msg := &Message{From: "a", To: "b"}
	targets := topo.Route("a", msg, members)
	assert.Equal(t, []string{"b"}, targets)
}

func TestMapReduceTopology_Name(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2"},
	}
	assert.Equal(t, "map-reduce", topo.Name())
}

func TestMapReduceTopology_Route_DispatcherFansOut(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2", "w3"},
	}
	members := []string{"d", "m", "w1", "w2", "w3"}

	msg := &Message{From: "d"}
	targets := topo.Route("d", msg, members)
	assert.Equal(t, []string{"w1", "w2", "w3"}, targets)
}

func TestMapReduceTopology_Route_WorkerToMerger(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2"},
	}
	members := []string{"d", "m", "w1", "w2"}

	msg := &Message{From: "w1"}
	targets := topo.Route("w1", msg, members)
	assert.Equal(t, []string{"m"}, targets)
}

func TestMapReduceTopology_Route_MergerToDispatcher(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1"},
	}
	members := []string{"d", "m", "w1"}

	msg := &Message{From: "m"}
	targets := topo.Route("m", msg, members)
	assert.Equal(t, []string{"d"}, targets)
}

func TestMapReduceTopology_Route_ExplicitRecipient(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1"},
	}
	members := []string{"d", "m", "w1"}

	msg := &Message{From: "d", To: "m"}
	targets := topo.Route("d", msg, members)
	assert.Equal(t, []string{"m"}, targets)
}

// Test all topologies implement the Topology interface
func TestTopology_InterfaceCompliance(t *testing.T) {
	topologies := []Topology{
		&LeaderTeammate{LeaderName: "lead"},
		&Pipeline{Stages: []string{"a", "b"}},
		&PeerRing{Ring: []string{"a", "b"}},
		&SupervisorTree{Parent: map[string]string{}, Children: map[string][]string{}},
		&BlackboardTopology{Board: NewBlackboard()},
		&MapReduceTopology{Dispatcher: "d", Merger: "m", Workers: []string{"w"}},
	}

	for _, topo := range topologies {
		assert.NotEmpty(t, topo.Name(), "topology name should not be empty")
		// Ensure OnMemberJoin/Leave don't panic
		topo.OnMemberJoin("test")
		topo.OnMemberLeave("test")
	}
}

// Test NextTask returns nil/empty for all topologies (none auto-assign)
func TestTopology_NextTask_NoAutoAssignment(t *testing.T) {
	topologies := []Topology{
		&LeaderTeammate{LeaderName: "lead"},
		&Pipeline{Stages: []string{"a"}},
		&PeerRing{Ring: []string{"a"}},
		&SupervisorTree{Parent: map[string]string{}, Children: map[string][]string{}},
		&BlackboardTopology{Board: NewBlackboard()},
		&MapReduceTopology{Dispatcher: "d", Merger: "m", Workers: []string{"w"}},
	}

	for _, topo := range topologies {
		result := topo.NextTask(nil, nil)
		assert.Nil(t, result, "NextTask for %s should return nil", topo.Name())
	}
}
