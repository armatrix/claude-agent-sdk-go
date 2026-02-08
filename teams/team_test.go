package teams

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

func TestNew_DefaultTopology(t *testing.T) {
	team := New("test-team")
	assert.NotEmpty(t, team.ID())
	assert.Equal(t, "test-team", team.Name())
	assert.NotNil(t, team.TaskList())
	assert.NotNil(t, team.Bus())
}

func TestNew_WithLeaderTeammateTopology(t *testing.T) {
	topo := &LeaderTeammate{LeaderName: "boss"}
	team := New("my-team", WithTopology(topo))
	assert.Equal(t, "my-team", team.Name())
	assert.Equal(t, topo, team.topology)
}

func TestNew_WithPipelineTopology(t *testing.T) {
	topo := &Pipeline{Stages: []string{"stage1", "stage2", "stage3"}}
	team := New("pipeline-team", WithTopology(topo))
	assert.Equal(t, "pipeline", team.topology.Name())
}

func TestNew_WithPeerRingTopology(t *testing.T) {
	topo := &PeerRing{Ring: []string{"a", "b", "c"}}
	team := New("ring-team", WithTopology(topo))
	assert.Equal(t, "peer-ring", team.topology.Name())
}

func TestNew_WithMemberDefs(t *testing.T) {
	team := New("team-with-members",
		WithMember("worker-1"),
		WithMember("worker-2"),
	)
	assert.Equal(t, "team-with-members", team.Name())
	assert.Len(t, team.opts.memberDefs, 2)
}

func TestNew_WithLeadAgent(t *testing.T) {
	team := New("team",
		WithLeadAgent(agent.WithSystemPrompt("You are the leader")),
	)
	assert.NotNil(t, team.opts.leadOpts)
}

func TestTeam_Start_ReturnsStream(t *testing.T) {
	team := New("test-team",
		WithLeadAgent(agent.WithMaxTurns(1)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	stream := team.Start(ctx, "hello")
	assert.NotNil(t, stream)

	// The stream should eventually close when the context expires
	// We don't need events to actually come through since we don't have a real API key
	<-ctx.Done()
	team.Shutdown()
}

func TestTeam_Start_CreatesLeader(t *testing.T) {
	team := New("test-team",
		WithTopology(&LeaderTeammate{LeaderName: "boss"}),
		WithLeadAgent(agent.WithMaxTurns(1)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = team.Start(ctx, "hello")

	// Leader should be created
	assert.NotNil(t, team.Lead())
	assert.Equal(t, "boss", team.Lead().Name())
	assert.Equal(t, RoleLead, team.Lead().Role())

	// Leader should be in members map
	members := team.Members()
	assert.Contains(t, members, "boss")

	team.Shutdown()
}

func TestTeam_SpawnMember_AddsToTeam(t *testing.T) {
	team := New("test-team",
		WithLeadAgent(agent.WithMaxTurns(1)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = team.Start(ctx, "hello")

	err := team.SpawnMember("worker-1", WithMemberAgentOptions(agent.WithMaxTurns(1)))
	require.NoError(t, err)

	members := team.Members()
	assert.Contains(t, members, "worker-1")
	assert.Equal(t, RoleTeammate, members["worker-1"].Role())

	team.Shutdown()
}

func TestTeam_SpawnMember_DuplicateNameError(t *testing.T) {
	team := New("test-team",
		WithLeadAgent(agent.WithMaxTurns(1)),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = team.Start(ctx, "hello")

	err := team.SpawnMember("worker-1")
	require.NoError(t, err)

	err = team.SpawnMember("worker-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	team.Shutdown()
}

func TestTeam_Shutdown(t *testing.T) {
	team := New("test-team",
		WithLeadAgent(agent.WithMaxTurns(1)),
	)

	ctx := context.Background()
	_ = team.Start(ctx, "hello")

	err := team.Shutdown()
	assert.NoError(t, err)
}

func TestTeam_ID_IsUnique(t *testing.T) {
	team1 := New("team-1")
	team2 := New("team-2")
	assert.NotEqual(t, team1.ID(), team2.ID())
}

func TestMemberOption_WithMemberAgentOptions(t *testing.T) {
	var mo memberOptions
	opt := WithMemberAgentOptions(agent.WithMaxTurns(5))
	opt(&mo)
	assert.Len(t, mo.agentOpts, 1)
}
