package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPlugins_Empty(t *testing.T) {
	dir := t.TempDir()
	plugins, err := LoadPlugins(dir)
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestLoadPlugins_NonExistentDir(t *testing.T) {
	plugins, err := LoadPlugins("/tmp/nonexistent-plugin-dir-xyz")
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestLoadPlugins_WithCommands(t *testing.T) {
	dir := t.TempDir()

	// Create plugin with commands
	pluginDir := filepath.Join(dir, "my-plugin")
	commandsDir := filepath.Join(pluginDir, "commands")
	require.NoError(t, os.MkdirAll(commandsDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(commandsDir, "commit.md"),
		[]byte("Create a git commit with a descriptive message"),
		0644,
	))

	plugins, err := LoadPlugins(dir)
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	p := plugins[0]
	assert.Equal(t, "my-plugin", p.Name)
	assert.Len(t, p.Commands, 1)
	assert.Equal(t, "commit", p.Commands["commit"].Name)
	assert.Contains(t, p.Commands["commit"].Content, "git commit")
}

func TestLoadPlugins_WithAgents(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "test-plugin")
	agentsDir := filepath.Join(pluginDir, "agents")
	require.NoError(t, os.MkdirAll(agentsDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(agentsDir, "reviewer.md"),
		[]byte("You are a code review specialist"),
		0644,
	))

	plugins, err := LoadPlugins(dir)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Len(t, plugins[0].AgentDefs, 1)
	assert.Equal(t, "reviewer", plugins[0].AgentDefs["reviewer"].Name)
}

func TestLoadPlugins_WithSkills(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "skill-plugin")
	skillsDir := filepath.Join(pluginDir, "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillsDir, "tdd.md"),
		[]byte("Test-driven development workflow"),
		0644,
	))

	plugins, err := LoadPlugins(dir)
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Len(t, plugins[0].Skills, 1)
}

func TestLoadPlugins_WithManifest(t *testing.T) {
	dir := t.TempDir()

	pluginDir := filepath.Join(dir, "manifest-plugin")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	manifest := `{
		"mcpServers": {
			"github": {
				"command": "npx",
				"args": ["@modelcontextprotocol/server-github"],
				"transport": "stdio"
			}
		}
	}`
	require.NoError(t, os.WriteFile(
		filepath.Join(pluginDir, "plugin.json"),
		[]byte(manifest),
		0644,
	))

	plugins, err := LoadPlugins(dir)
	require.NoError(t, err)
	require.Len(t, plugins, 1)

	p := plugins[0]
	assert.Equal(t, "manifest-plugin", p.Name)
	assert.Len(t, p.MCPServers, 1)
	assert.Equal(t, "npx", p.MCPServers["github"].Command)
}

func TestLoadPlugins_EmptyPlugin_Skipped(t *testing.T) {
	dir := t.TempDir()

	// Create an empty plugin directory (no commands, agents, skills, or manifest)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "empty-plugin"), 0755))

	plugins, err := LoadPlugins(dir)
	require.NoError(t, err)
	assert.Empty(t, plugins)
}
