// Package config handles settings and skills loading for the agent SDK.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings holds merged configuration from multiple sources.
// Later sources override earlier ones (user < project < local).
type Settings struct {
	Model           string            `json:"model,omitempty"`
	SystemPrompt    string            `json:"systemPrompt,omitempty"`
	MaxTurns        int               `json:"maxTurns,omitempty"`
	MaxBudgetUSD    float64           `json:"maxBudgetUSD,omitempty"`
	BuiltinTools    []string          `json:"builtinTools,omitempty"`
	DisabledTools   []string          `json:"disabledTools,omitempty"`
	CustomSettings  map[string]any    `json:"custom,omitempty"`
	PermissionMode  string            `json:"permissionMode,omitempty"`
}

// LoadSettings merges settings from multiple JSON file paths.
// Later paths override earlier ones. Missing files are silently skipped.
func LoadSettings(paths ...string) (*Settings, error) {
	merged := &Settings{
		CustomSettings: make(map[string]any),
	}

	for _, path := range paths {
		s, err := loadSettingsFile(path)
		if err != nil {
			continue // Skip missing or invalid files
		}
		mergeSettings(merged, s)
	}

	return merged, nil
}

// DefaultSettingsPaths returns the standard settings file search paths.
func DefaultSettingsPaths(projectDir string) []string {
	home, _ := os.UserHomeDir()
	var paths []string

	// User-level settings
	if home != "" {
		paths = append(paths, filepath.Join(home, ".claude", "settings.json"))
	}

	// Project-level settings
	if projectDir != "" {
		paths = append(paths,
			filepath.Join(projectDir, ".claude", "settings.json"),
			filepath.Join(projectDir, "CLAUDE.md"),
		)
	}

	return paths
}

func loadSettingsFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func mergeSettings(dst, src *Settings) {
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.SystemPrompt != "" {
		dst.SystemPrompt = src.SystemPrompt
	}
	if src.MaxTurns > 0 {
		dst.MaxTurns = src.MaxTurns
	}
	if src.MaxBudgetUSD > 0 {
		dst.MaxBudgetUSD = src.MaxBudgetUSD
	}
	if len(src.BuiltinTools) > 0 {
		dst.BuiltinTools = src.BuiltinTools
	}
	if len(src.DisabledTools) > 0 {
		dst.DisabledTools = src.DisabledTools
	}
	if src.PermissionMode != "" {
		dst.PermissionMode = src.PermissionMode
	}
	for k, v := range src.CustomSettings {
		if dst.CustomSettings == nil {
			dst.CustomSettings = make(map[string]any)
		}
		dst.CustomSettings[k] = v
	}
}
