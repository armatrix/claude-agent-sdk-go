package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Plugin represents a loaded plugin with its commands, agent definitions,
// MCP server configs, and skill files.
type Plugin struct {
	// Name is the plugin identifier (derived from directory name).
	Name string `json:"name"`

	// Dir is the absolute path to the plugin directory.
	Dir string `json:"-"`

	// Commands maps slash command names to their markdown content.
	Commands map[string]*Command `json:"commands,omitempty"`

	// AgentDefs maps agent names to their configuration.
	AgentDefs map[string]*AgentDef `json:"agents,omitempty"`

	// MCPServers maps server names to their configuration.
	MCPServers map[string]*MCPServerConfig `json:"mcpServers,omitempty"`

	// Skills lists the paths of .md skill files.
	Skills []string `json:"skills,omitempty"`
}

// Command is a slash command loaded from a .md file.
type Command struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// AgentDef defines a custom agent configuration within a plugin.
type AgentDef struct {
	Name         string `json:"name"`
	Model        string `json:"model,omitempty"`
	Instructions string `json:"instructions,omitempty"`
}

// MCPServerConfig defines an MCP server within a plugin.
type MCPServerConfig struct {
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Transport string            `json:"transport,omitempty"`
}

// LoadPlugins scans the given directories for plugin definitions.
// Each subdirectory is treated as a plugin. A plugin is identified by
// having a plugin.json manifest or containing commands/, agents/, or skills/ subdirs.
func LoadPlugins(dirs ...string) ([]*Plugin, error) {
	var plugins []*Plugin

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("plugin: read dir %s: %w", dir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pluginDir := filepath.Join(dir, entry.Name())
			p, err := loadPlugin(pluginDir, entry.Name())
			if err != nil {
				return nil, fmt.Errorf("plugin: load %s: %w", entry.Name(), err)
			}
			if p != nil {
				plugins = append(plugins, p)
			}
		}
	}

	return plugins, nil
}

func loadPlugin(dir, name string) (*Plugin, error) {
	p := &Plugin{
		Name:       name,
		Dir:        dir,
		Commands:   make(map[string]*Command),
		AgentDefs:  make(map[string]*AgentDef),
		MCPServers: make(map[string]*MCPServerConfig),
	}

	// Try loading plugin.json manifest
	manifestPath := filepath.Join(dir, "plugin.json")
	if data, err := os.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(data, p); err != nil {
			return nil, fmt.Errorf("parse plugin.json: %w", err)
		}
		p.Name = name
		p.Dir = dir
	}

	// Load commands from commands/ directory
	commandsDir := filepath.Join(dir, "commands")
	if entries, err := os.ReadDir(commandsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(commandsDir, e.Name()))
			if err != nil {
				continue
			}
			cmdName := strings.TrimSuffix(e.Name(), ".md")
			p.Commands[cmdName] = &Command{
				Name:    cmdName,
				Content: string(content),
			}
		}
	}

	// Load agent definitions from agents/ directory
	agentsDir := filepath.Join(dir, "agents")
	if entries, err := os.ReadDir(agentsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
			if err != nil {
				continue
			}
			agentName := strings.TrimSuffix(e.Name(), ".md")
			p.AgentDefs[agentName] = &AgentDef{
				Name:         agentName,
				Instructions: string(content),
			}
		}
	}

	// Load skills from skills/ directory
	skillsDir := filepath.Join(dir, "skills")
	if entries, err := os.ReadDir(skillsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			p.Skills = append(p.Skills, filepath.Join(skillsDir, e.Name()))
		}
	}

	// Only return if plugin has content
	if len(p.Commands) == 0 && len(p.AgentDefs) == 0 && len(p.MCPServers) == 0 && len(p.Skills) == 0 {
		return nil, nil
	}

	return p, nil
}
