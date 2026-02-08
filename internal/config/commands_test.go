package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCommands_Empty(t *testing.T) {
	dir := t.TempDir()
	cmds, err := LoadCommands(dir)
	require.NoError(t, err)
	assert.Empty(t, cmds)
}

func TestLoadCommands_NonExistent(t *testing.T) {
	cmds, err := LoadCommands("/tmp/nonexistent-commands-dir-xyz")
	require.NoError(t, err)
	assert.Empty(t, cmds)
}

func TestLoadCommands_LoadsMdFiles(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "commit.md"),
		[]byte("Create a git commit"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "review.md"),
		[]byte("Review the PR"),
		0644,
	))
	// Non-.md files should be skipped
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "README.txt"),
		[]byte("Not a command"),
		0644,
	))

	cmds, err := LoadCommands(dir)
	require.NoError(t, err)
	assert.Len(t, cmds, 2)

	cmdMap := make(map[string]Command)
	for _, c := range cmds {
		cmdMap[c.Name] = c
	}
	assert.Contains(t, cmdMap, "commit")
	assert.Equal(t, "Create a git commit", cmdMap["commit"].Content)
	assert.Contains(t, cmdMap, "review")
}

func TestLoadCommands_LaterDirOverrides(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	require.NoError(t, os.WriteFile(
		filepath.Join(dir1, "commit.md"),
		[]byte("v1"),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir2, "commit.md"),
		[]byte("v2"),
		0644,
	))

	cmds, err := LoadCommands(dir1, dir2)
	require.NoError(t, err)
	assert.Len(t, cmds, 1)
	assert.Equal(t, "v2", cmds[0].Content)
}
