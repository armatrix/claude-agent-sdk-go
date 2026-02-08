package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Command represents a slash command loaded from a .md file.
type Command struct {
	// Name is the command name (e.g. "commit", "review-pr").
	Name string

	// Content is the markdown template content.
	Content string

	// FilePath is the absolute path to the source .md file.
	FilePath string
}

// LoadCommands scans directories for .md slash command files.
// Files are named after the command (e.g. commit.md -> /commit).
// Later directories override earlier ones for the same command name.
func LoadCommands(dirs ...string) ([]Command, error) {
	seen := make(map[string]Command)

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}

			filePath := filepath.Join(dir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			name := strings.TrimSuffix(entry.Name(), ".md")
			seen[name] = Command{
				Name:     name,
				Content:  string(content),
				FilePath: filePath,
			}
		}
	}

	commands := make([]Command, 0, len(seen))
	for _, cmd := range seen {
		commands = append(commands, cmd)
	}
	return commands, nil
}
