package config

// Presets maps preset names to their system prompt content.
// These are short built-in prompts that can be selected via WithSystemPromptPreset.
var Presets = map[string]string{
	"claude_code": "You are an AI assistant with access to tools for reading, writing, and editing files, running shell commands, and searching the codebase. Use tools to help the user with software engineering tasks.",
}

// GetPreset returns the system prompt for the given preset name.
// Returns empty string and false if the preset is not found.
func GetPreset(name string) (string, bool) {
	content, ok := Presets[name]
	return content, ok
}
