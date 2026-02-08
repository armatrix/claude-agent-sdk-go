package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSkills_SingleDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "commit.md"), []byte("# Commit\nGit commit helper"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "review.md"), []byte("# Review\nCode review"), 0o644))

	skills, err := LoadSkills(dir)
	require.NoError(t, err)
	assert.Len(t, skills, 2)

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	assert.True(t, names["commit"])
	assert.True(t, names["review"])
}

func TestLoadSkills_SkipsNonMD(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skill.md"), []byte("valid"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignored"), 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))

	skills, err := LoadSkills(dir)
	require.NoError(t, err)
	assert.Len(t, skills, 1)
	assert.Equal(t, "skill", skills[0].Name)
}

func TestLoadSkills_MissingDirSkipped(t *testing.T) {
	skills, err := LoadSkills("/nonexistent/dir")
	require.NoError(t, err)
	assert.Empty(t, skills)
}

func TestLoadSkills_MultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir1, "a.md"), []byte("skill A"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "b.md"), []byte("skill B"), 0o644))

	skills, err := LoadSkills(dir1, dir2)
	require.NoError(t, err)
	assert.Len(t, skills, 2)
}

func TestFormatSkillsPrompt_Empty(t *testing.T) {
	result := FormatSkillsPrompt(nil)
	assert.Equal(t, "", result)
}

func TestFormatSkillsPrompt_WithSkills(t *testing.T) {
	skills := []Skill{
		{Name: "commit", Content: "Git commit helper"},
		{Name: "review", Content: "Code review helper"},
	}

	result := FormatSkillsPrompt(skills)
	assert.Contains(t, result, "# Available Skills")
	assert.Contains(t, result, "## commit")
	assert.Contains(t, result, "Git commit helper")
	assert.Contains(t, result, "## review")
	assert.Contains(t, result, "Code review helper")
}
