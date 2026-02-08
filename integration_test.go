package agent_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	agent "github.com/armatrix/claude-agent-sdk-go"
	"github.com/armatrix/claude-agent-sdk-go/session"
	"github.com/armatrix/claude-agent-sdk-go/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


// TestIntegration_FullAgentRun_WithAPI performs a real API call.
// Requires ANTHROPIC_API_KEY environment variable.
func TestIntegration_FullAgentRun_WithAPI(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	a := agent.NewAgent(
		agent.WithModel(anthropic.ModelClaudeHaiku4_5_20251001), // Use cheapest model for testing
		agent.WithMaxTurns(3),
	)

	// Register builtin tools
	tools.RegisterAll(a.Tools())

	ctx := context.Background()
	stream := a.Run(ctx, "Read the file go.mod in the current directory and tell me the module name. Reply with ONLY the module name, nothing else.")

	var events []agent.Event
	var streamText strings.Builder

	for stream.Next() {
		evt := stream.Current()
		events = append(events, evt)

		switch e := evt.(type) {
		case *agent.StreamEvent:
			streamText.WriteString(e.Delta)
		}
	}

	require.NoError(t, stream.Err())

	// Should have at least: system + assistant(s) + result
	assert.GreaterOrEqual(t, len(events), 3)

	// Find result event
	var result *agent.ResultEvent
	for _, evt := range events {
		if r, ok := evt.(*agent.ResultEvent); ok {
			result = r
			break
		}
	}

	require.NotNil(t, result, "should have a result event")
	assert.Equal(t, "success", result.Subtype)
	assert.False(t, result.IsError)
	assert.Greater(t, result.Usage.InputTokens, int64(0))
	assert.Greater(t, result.Usage.OutputTokens, int64(0))

	// The module name should appear somewhere in the output
	assert.Contains(t, streamText.String(), "claude-agent-sdk-go")
}

// TestIntegration_Client_MultiTurn verifies Client maintains session across queries.
// Requires ANTHROPIC_API_KEY environment variable.
func TestIntegration_Client_MultiTurn(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping integration test")
	}

	client := agent.NewClient(
		agent.WithModel(anthropic.ModelClaudeHaiku4_5_20251001),
		agent.WithMaxTurns(2),
	)
	defer client.Close()

	ctx := context.Background()

	// First query
	stream1 := client.Query(ctx, "Remember this number: 42. Reply with just 'OK'.")
	for stream1.Next() {
		// drain
	}
	require.NoError(t, stream1.Err())

	// Session should have messages now
	session := client.Session()
	assert.Greater(t, len(session.Messages), 0)

	// Second query using same session
	stream2 := client.Query(ctx, "What number did I ask you to remember? Reply with ONLY the number.")
	var text strings.Builder
	for stream2.Next() {
		if e, ok := stream2.Current().(*agent.StreamEvent); ok {
			text.WriteString(e.Delta)
		}
	}
	require.NoError(t, stream2.Err())

	assert.Contains(t, text.String(), "42")
}

// TestIntegration_StreamIterator verifies the stream iterator contract.
func TestIntegration_StreamIterator(t *testing.T) {
	// Create an agent with no API key — this will fail at API call
	// but we can verify the stream produces proper error handling
	a := agent.NewAgent(
		agent.WithModel(anthropic.ModelClaudeOpus4_6),
		agent.WithMaxTurns(1),
	)

	ctx := context.Background()
	stream := a.Run(ctx, "hello")

	var gotSystem, gotResult bool
	for stream.Next() {
		evt := stream.Current()
		switch evt.Type() {
		case agent.EventSystem:
			gotSystem = true
		case agent.EventResult:
			gotResult = true
		}
	}

	// System event should always be emitted
	assert.True(t, gotSystem, "should emit system event")
	// Result event should be emitted (even on error)
	assert.True(t, gotResult, "should emit result event")
}

// TestIntegration_AgentWithTools verifies tool registration and listing.
func TestIntegration_AgentWithTools(t *testing.T) {
	a := agent.NewAgent()

	tools.RegisterAll(a.Tools())

	names := a.Tools().Names()
	assert.Contains(t, names, "Read")
	assert.Contains(t, names, "Write")
	assert.Contains(t, names, "Edit")
	assert.Contains(t, names, "Bash")
	assert.Contains(t, names, "Glob")
	assert.Contains(t, names, "Grep")

	// ListForAPI should produce valid tool params
	apiTools := a.Tools().ListForAPI()
	assert.Len(t, apiTools, 6)

	for _, tool := range apiTools {
		assert.NotNil(t, tool.OfTool)
		assert.NotEmpty(t, tool.OfTool.Name)
	}
}

// TestIntegration_DefaultOptions verifies default configuration.
func TestIntegration_DefaultOptions(t *testing.T) {
	a := agent.NewAgent()

	// Model() is the exported accessor for the configured model
	assert.Equal(t, agent.DefaultModel, a.Model())

	// Custom model overrides default
	a2 := agent.NewAgent(agent.WithModel(anthropic.ModelClaudeHaiku4_5_20251001))
	assert.Equal(t, anthropic.ModelClaudeHaiku4_5_20251001, a2.Model())
}

// --- Phase 2 Integration Tests ---

// TestIntegration_SystemPromptOption verifies system prompt is wired through.
func TestIntegration_SystemPromptOption(t *testing.T) {
	a := agent.NewAgent(
		agent.WithSystemPrompt("You are a pirate. Always respond in pirate speak."),
	)
	require.NotNil(t, a)
	assert.Equal(t, "You are a pirate. Always respond in pirate speak.", a.Options().SystemPromptText())
}

// TestIntegration_SessionStore verifies session store wiring.
func TestIntegration_SessionStore(t *testing.T) {
	store := session.NewMemoryStore()

	client := agent.NewClient(
		agent.WithSessionStore(store),
		agent.WithMaxTurns(1),
	)
	defer client.Close()

	// Session should be accessible
	session := client.Session()
	assert.NotNil(t, session)
	assert.NotEmpty(t, session.ID)
}

// TestIntegration_SettingsLoading verifies settings from files are applied.
func TestIntegration_SettingsLoading(t *testing.T) {
	dir := t.TempDir()
	settingsFile := dir + "/settings.json"
	require.NoError(t, os.WriteFile(settingsFile, []byte(`{"maxTurns":42}`), 0o644))

	a := agent.NewAgent(agent.WithSettingSources(settingsFile))
	assert.Equal(t, 42, a.Options().MaxTurnsValue())
}

// TestIntegration_SkillsLoading verifies skills directories are loaded and prepended.
func TestIntegration_SkillsLoading(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(dir+"/greet.md", []byte("Always greet the user warmly."), 0o644))

	a := agent.NewAgent(
		agent.WithSystemPrompt("Base instructions"),
		agent.WithSkillDirs(dir),
	)

	prompt := a.Options().SystemPromptText()
	assert.Contains(t, prompt, "Always greet the user warmly.")
	assert.Contains(t, prompt, "Base instructions")
}

// TestIntegration_MemoryStoreRoundTrip verifies memory store save/load.
func TestIntegration_MemoryStoreRoundTrip(t *testing.T) {
	store := session.NewMemoryStore()
	ctx := context.Background()

	session := agent.NewSession()
	session.Messages = append(session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("hello")))

	require.NoError(t, store.Save(ctx, session))

	loaded, err := store.Load(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, loaded.ID)
	assert.Len(t, loaded.Messages, 1)
}

// TestIntegration_FileStoreRoundTrip verifies file store save/load.
func TestIntegration_FileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := session.NewFileStore(dir)
	require.NoError(t, err)

	ctx := context.Background()

	session := agent.NewSession()
	session.Messages = append(session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("test")))

	require.NoError(t, store.Save(ctx, session))

	loaded, err := store.Load(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, loaded.ID)
	assert.Len(t, loaded.Messages, 1)
}

// TestIntegration_SessionClone verifies session forking.
func TestIntegration_SessionClone(t *testing.T) {
	session := agent.NewSession()
	session.Messages = append(session.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("original")))

	// Small sleep to ensure different timestamp-based ID
	time.Sleep(time.Millisecond)
	clone := session.Clone()

	// Different IDs
	assert.NotEqual(t, session.ID, clone.ID)
	// Same message count
	assert.Len(t, clone.Messages, 1)
	// Modifying clone doesn't affect original
	clone.Messages = append(clone.Messages,
		anthropic.NewUserMessage(anthropic.NewTextBlock("added")))
	assert.Len(t, session.Messages, 1)
	assert.Len(t, clone.Messages, 2)
}

// TestIntegration_OutputFormatOption verifies structured output option wiring.
func TestIntegration_OutputFormatOption(t *testing.T) {
	type Response struct {
		Summary string `json:"summary" jsonschema:"required"`
		Score   int    `json:"score" jsonschema:"required"`
	}

	a := agent.NewAgent(
		agent.WithOutputFormatType[Response]("analysis_output"),
	)

	require.NotNil(t, a.Options().OutputFormatName())
	assert.Equal(t, "analysis_output", a.Options().OutputFormatName())
}

// TestIntegration_ConfigurableBuiltinTools verifies extended tool registration.
func TestIntegration_ConfigurableBuiltinTools(t *testing.T) {
	a := agent.NewAgent()
	tools.RegisterAll(a.Tools())

	// Register configurable tools
	tools.RegisterConfigurable(a.Tools(), tools.BuiltinOptions{
		AskCallback: func(ctx context.Context, question string, options []tools.AskOption) (string, error) {
			return "yes", nil
		},
		PlanCallback: func(ctx context.Context, plan string) error {
			return nil
		},
	})

	names := a.Tools().Names()
	assert.Contains(t, names, "Read")
	assert.Contains(t, names, "AskUserQuestion")
	assert.Contains(t, names, "TodoWrite")
	assert.Contains(t, names, "ExitPlanMode")
}

// TestIntegration_BackwardCompat_ExistingBehavior verifies Phase 1 behavior unchanged.
func TestIntegration_BackwardCompat_ExistingBehavior(t *testing.T) {
	// Agent with no Phase 2 options should work exactly as before
	a := agent.NewAgent()

	assert.Equal(t, agent.DefaultModel, a.Model())
	assert.NotNil(t, a.Tools())

	// Register only Phase 1 tools
	tools.RegisterAll(a.Tools())
	apiTools := a.Tools().ListForAPI()
	assert.Len(t, apiTools, 6) // Read, Write, Edit, Bash, Glob, Grep
}
