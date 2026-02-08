package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSettings_SingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	s := Settings{
		Model:      "claude-sonnet-4-5",
		MaxTurns:   10,
		MaxBudgetUSD: 5.0,
	}
	data, _ := json.Marshal(s)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	result, err := LoadSettings(path)
	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-5", result.Model)
	assert.Equal(t, 10, result.MaxTurns)
	assert.Equal(t, 5.0, result.MaxBudgetUSD)
}

func TestLoadSettings_MergeOrder(t *testing.T) {
	dir := t.TempDir()

	// User settings (loaded first)
	userPath := filepath.Join(dir, "user.json")
	userData, _ := json.Marshal(Settings{Model: "claude-haiku", MaxTurns: 5})
	require.NoError(t, os.WriteFile(userPath, userData, 0o644))

	// Project settings (loaded second, overrides user)
	projPath := filepath.Join(dir, "project.json")
	projData, _ := json.Marshal(Settings{Model: "claude-sonnet", SystemPrompt: "Be helpful"})
	require.NoError(t, os.WriteFile(projPath, projData, 0o644))

	result, err := LoadSettings(userPath, projPath)
	require.NoError(t, err)

	assert.Equal(t, "claude-sonnet", result.Model, "project should override user")
	assert.Equal(t, 5, result.MaxTurns, "user value preserved when project doesn't set it")
	assert.Equal(t, "Be helpful", result.SystemPrompt, "project value applied")
}

func TestLoadSettings_MissingFileSkipped(t *testing.T) {
	result, err := LoadSettings("/nonexistent/path.json")
	require.NoError(t, err)
	assert.Equal(t, "", result.Model)
}

func TestLoadSettings_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	result, err := LoadSettings(path)
	require.NoError(t, err)
	assert.Equal(t, "", result.Model) // Invalid file skipped
}

func TestLoadSettings_CustomSettings(t *testing.T) {
	dir := t.TempDir()

	path1 := filepath.Join(dir, "a.json")
	data1, _ := json.Marshal(Settings{CustomSettings: map[string]any{"key1": "val1"}})
	require.NoError(t, os.WriteFile(path1, data1, 0o644))

	path2 := filepath.Join(dir, "b.json")
	data2, _ := json.Marshal(Settings{CustomSettings: map[string]any{"key2": "val2"}})
	require.NoError(t, os.WriteFile(path2, data2, 0o644))

	result, err := LoadSettings(path1, path2)
	require.NoError(t, err)
	assert.Equal(t, "val1", result.CustomSettings["key1"])
	assert.Equal(t, "val2", result.CustomSettings["key2"])
}

func TestDefaultSettingsPaths(t *testing.T) {
	paths := DefaultSettingsPaths("/myproject")
	assert.NotEmpty(t, paths)
	// Should include project-level path
	found := false
	for _, p := range paths {
		if filepath.Base(filepath.Dir(p)) == ".claude" || filepath.Base(p) == "CLAUDE.md" {
			found = true
		}
	}
	assert.True(t, found, "should include project settings paths")
}
