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

// Test NextTask returns nil when no tasks or members provided
func TestTopology_NextTask_NilInputs(t *testing.T) {
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
		assert.Nil(t, result, "NextTask for %s should return nil with nil inputs", topo.Name())
	}
}

// --- Pipeline NextTask ---

func TestPipeline_NextTask_AssignsToEarliestStage(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b", "c"}}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{
		newIdleMember("a"),
		newIdleMember("b"),
		newIdleMember("c"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 2)
	assert.Equal(t, "t1", assignments[0].TaskID)
	assert.Equal(t, "a", assignments[0].MemberName) // earliest stage first
	assert.Equal(t, "t2", assignments[1].TaskID)
	assert.Equal(t, "b", assignments[1].MemberName)
}

func TestPipeline_NextTask_SkipsBusyMembers(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b"}}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending},
	}
	busyA := newIdleMember("a")
	busyA.SetStatus(MemberWorking)
	members := []*Member{busyA, newIdleMember("b")}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "b", assignments[0].MemberName)
}

func TestPipeline_NextTask_SkipsOwnedTasks(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a"}}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending, Owner: "someone"},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{newIdleMember("a")}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "t2", assignments[0].TaskID)
}

func TestPipeline_OnMemberLeave(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b", "c"}}
	_ = topo.activeStages() // initialize
	topo.OnMemberLeave("b")
	stages := topo.activeStages()
	assert.Equal(t, []string{"a", "c"}, stages)
}

func TestPipeline_OnMemberJoin_RejoinsKnownMember(t *testing.T) {
	topo := &Pipeline{Stages: []string{"a", "b"}}
	_ = topo.activeStages() // initialize
	topo.OnMemberLeave("b")
	topo.OnMemberJoin("b")
	stages := topo.activeStages()
	assert.Contains(t, stages, "b")
}

// --- PeerRing NextTask ---

func TestPeerRing_NextTask_RoundRobin(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{
		newIdleMember("a"),
		newIdleMember("b"),
		newIdleMember("c"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 2)
	// Round-robin: a gets t1, b gets t2
	assert.Equal(t, "a", assignments[0].MemberName)
	assert.Equal(t, "b", assignments[1].MemberName)
}

func TestPeerRing_NextTask_WrapsAround(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b"}}
	// First call uses up a and b
	tasks1 := []*Task{
		{ID: "t1", Status: TaskPending},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{newIdleMember("a"), newIdleMember("b")}
	topo.NextTask(tasks1, members)

	// Second call should start from where it left off
	tasks2 := []*Task{
		{ID: "t3", Status: TaskPending},
	}
	assignments := topo.NextTask(tasks2, members)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "a", assignments[0].MemberName) // wraps back to a
}

func TestPeerRing_OnMemberLeave(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	_ = topo.currentRing() // initialize
	topo.OnMemberLeave("b")
	ring := topo.currentRing()
	assert.Equal(t, []string{"a", "c"}, ring)
}

// --- SupervisorTree NextTask ---

func TestSupervisorTree_NextTask_AssignsToLeaves(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"child1": "root", "child2": "root"},
		Children: map[string][]string{"root": {"child1", "child2"}},
	}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{
		newIdleMember("root"),
		newIdleMember("child1"),
		newIdleMember("child2"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 2)
	// Only leaf nodes get assigned; root is a supervisor
	names := []string{assignments[0].MemberName, assignments[1].MemberName}
	assert.Contains(t, names, "child1")
	assert.Contains(t, names, "child2")
	assert.NotContains(t, names, "root")
}

func TestSupervisorTree_NextTask_SkipsSupervisors(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"mid": "root", "leaf": "mid"},
		Children: map[string][]string{"root": {"mid"}, "mid": {"leaf"}},
	}
	tasks := []*Task{{ID: "t1", Status: TaskPending}}
	members := []*Member{
		newIdleMember("root"),
		newIdleMember("mid"),
		newIdleMember("leaf"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "leaf", assignments[0].MemberName)
}

func TestSupervisorTree_OnMemberLeave_ReparentsOrphans(t *testing.T) {
	topo := &SupervisorTree{
		Parent:   map[string]string{"mid": "root", "leaf1": "mid", "leaf2": "mid"},
		Children: map[string][]string{"root": {"mid"}, "mid": {"leaf1", "leaf2"}},
	}

	topo.OnMemberLeave("mid")

	// leaf1 and leaf2 should now be children of root
	assert.Equal(t, "root", topo.Parent["leaf1"])
	assert.Equal(t, "root", topo.Parent["leaf2"])
	assert.Contains(t, topo.Children["root"], "leaf1")
	assert.Contains(t, topo.Children["root"], "leaf2")
	// mid should be removed
	_, hasMid := topo.Parent["mid"]
	assert.False(t, hasMid)
	assert.Nil(t, topo.Children["mid"])
}

// --- BlackboardTopology NextTask ---

func TestBlackboardTopology_NextTask_AssignsToIdle(t *testing.T) {
	topo := &BlackboardTopology{Board: NewBlackboard()}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{
		newIdleMember("expert1"),
		newIdleMember("expert2"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 2)
	assert.Equal(t, "expert1", assignments[0].MemberName)
	assert.Equal(t, "expert2", assignments[1].MemberName)
}

func TestBlackboardTopology_NextTask_SkipsBlockedTasks(t *testing.T) {
	topo := &BlackboardTopology{Board: NewBlackboard()}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending, BlockedBy: []string{"t0"}},
		{ID: "t0", Status: TaskInProgress}, // blocker not completed
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{newIdleMember("a")}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "t2", assignments[0].TaskID)
}

// --- MapReduce NextTask ---

func TestMapReduce_NextTask_AssignsToWorkersOnly(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2"},
	}
	tasks := []*Task{
		{ID: "t1", Status: TaskPending},
		{ID: "t2", Status: TaskPending},
	}
	members := []*Member{
		newIdleMember("d"),
		newIdleMember("m"),
		newIdleMember("w1"),
		newIdleMember("w2"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 2)
	names := []string{assignments[0].MemberName, assignments[1].MemberName}
	assert.Contains(t, names, "w1")
	assert.Contains(t, names, "w2")
	assert.NotContains(t, names, "d")
	assert.NotContains(t, names, "m")
}

func TestMapReduce_NextTask_SkipsBusyWorkers(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2"},
	}
	tasks := []*Task{{ID: "t1", Status: TaskPending}}
	busyW1 := newIdleMember("w1")
	busyW1.SetStatus(MemberWorking)
	members := []*Member{
		newIdleMember("d"),
		newIdleMember("m"),
		busyW1,
		newIdleMember("w2"),
	}

	assignments := topo.NextTask(tasks, members)
	assert.Len(t, assignments, 1)
	assert.Equal(t, "w2", assignments[0].MemberName)
}

func TestMapReduce_OnMemberLeave_RemovesWorker(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2", "w3"},
	}
	_ = topo.currentWorkers() // initialize
	topo.OnMemberLeave("w2")
	workers := topo.currentWorkers()
	assert.Equal(t, []string{"w1", "w3"}, workers)
}

func TestMapReduce_OnMemberJoin_RejoinsWorker(t *testing.T) {
	topo := &MapReduceTopology{
		Dispatcher: "d",
		Merger:     "m",
		Workers:    []string{"w1", "w2"},
	}
	_ = topo.currentWorkers() // initialize
	topo.OnMemberLeave("w2")
	topo.OnMemberJoin("w2")
	workers := topo.currentWorkers()
	assert.Contains(t, workers, "w2")
}

// --- Helpers ---

func newIdleMember(name string) *Member {
	m := &Member{name: name}
	m.SetStatus(MemberIdle)
	return m
}
