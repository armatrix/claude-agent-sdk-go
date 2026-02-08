package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill file.
type Skill struct {
	Name    string // Derived from filename (without extension)
	Content string // Raw markdown content
}

// LoadSkills reads all .md files from the given directories and returns them
// as skill definitions that can be injected into the system prompt.
func LoadSkills(dirs ...string) ([]Skill, error) {
	var skills []Skill

	for _, dir := range dirs {
		dirSkills, err := loadSkillsFromDir(dir)
		if err != nil {
			continue // Skip missing directories
		}
		skills = append(skills, dirSkills...)
	}

	return skills, nil
}

// FormatSkillsPrompt formats loaded skills into a string suitable for
// prepending to a system prompt.
func FormatSkillsPrompt(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("# Available Skills\n\n")

	for _, skill := range skills {
		sb.WriteString("## ")
		sb.WriteString(skill.Name)
		sb.WriteString("\n\n")
		sb.WriteString(skill.Content)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

func loadSkillsFromDir(dir string) ([]Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		skills = append(skills, Skill{
			Name:    name,
			Content: string(content),
		})
	}

	return skills, nil
}
