package tools

import (
	"context"
	"fmt"
	"strings"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

// SearchResult represents a single search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchFunc is an injectable search backend.
type SearchFunc func(ctx context.Context, query string) ([]SearchResult, error)

// WebSearchInput defines the input for the WebSearch tool.
type WebSearchInput struct {
	Query          string   `json:"query" jsonschema:"required,description=The search query"`
	AllowedDomains []string `json:"allowed_domains,omitempty" jsonschema:"description=Only include results from these domains"`
	BlockedDomains []string `json:"blocked_domains,omitempty" jsonschema:"description=Exclude results from these domains"`
}

// WebSearchTool performs web searches.
type WebSearchTool struct {
	Search SearchFunc // Required: injectable search backend
}

var _ agent.Tool[WebSearchInput] = (*WebSearchTool)(nil)

func (t *WebSearchTool) Name() string        { return "WebSearch" }
func (t *WebSearchTool) Description() string  { return "Search the web for information" }

func (t *WebSearchTool) Execute(ctx context.Context, input WebSearchInput) (*agent.ToolResult, error) {
	if input.Query == "" {
		return agent.ErrorResult("query is required"), nil
	}
	if t.Search == nil {
		return agent.ErrorResult("search backend not configured"), nil
	}

	results, err := t.Search(ctx, input.Query)
	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("search failed: %s", err.Error())), nil
	}

	// Filter by allowed/blocked domains
	filtered := filterResults(results, input.AllowedDomains, input.BlockedDomains)

	if len(filtered) == 0 {
		return agent.TextResult("No results found."), nil
	}

	// Format results
	var sb strings.Builder
	for i, r := range filtered {
		sb.WriteString(fmt.Sprintf("%d. [%s](%s)\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet))
	}

	return agent.TextResult(sb.String()), nil
}

func filterResults(results []SearchResult, allowed, blocked []string) []SearchResult {
	if len(allowed) == 0 && len(blocked) == 0 {
		return results
	}

	allowSet := make(map[string]bool, len(allowed))
	for _, d := range allowed {
		allowSet[strings.ToLower(d)] = true
	}

	blockSet := make(map[string]bool, len(blocked))
	for _, d := range blocked {
		blockSet[strings.ToLower(d)] = true
	}

	var filtered []SearchResult
	for _, r := range results {
		domain := extractDomain(r.URL)
		if len(allowSet) > 0 && !allowSet[domain] {
			continue
		}
		if blockSet[domain] {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func extractDomain(url string) string {
	u := strings.ToLower(url)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if idx := strings.Index(u, "/"); idx > 0 {
		u = u[:idx]
	}
	return u
}
