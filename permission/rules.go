package permission

import "path"

// Rule is a declarative permission rule with glob pattern matching.
type Rule struct {
	Pattern  string   // glob pattern, e.g. "mcp__context7__*", "Bash", "Edit"
	Decision Decision // Allow, Deny, or Ask
}

// MatchRules evaluates rules against a tool name.
// Evaluation order: deny rules, then ask rules, then allow rules.
// Returns (decision, matched). If no rule matches, matched is false.
func MatchRules(rules []Rule, toolName string) (Decision, bool) {
	var hasAsk, hasAllow bool

	for _, r := range rules {
		ok, err := path.Match(r.Pattern, toolName)
		if err != nil || !ok {
			continue
		}
		switch r.Decision {
		case Deny:
			return Deny, true
		case Ask:
			hasAsk = true
		case Allow:
			hasAllow = true
		}
	}

	if hasAsk {
		return Ask, true
	}
	if hasAllow {
		return Allow, true
	}
	return Allow, false
}
