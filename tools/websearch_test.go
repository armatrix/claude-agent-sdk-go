package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSearchTool_Name(t *testing.T) {
	tool := &WebSearchTool{}
	assert.Equal(t, "WebSearch", tool.Name())
}

func TestWebSearchTool_Description(t *testing.T) {
	tool := &WebSearchTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestWebSearchTool_Execute_Success(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Go Programming", URL: "https://go.dev", Snippet: "The Go language"},
				{Title: "Go Tutorial", URL: "https://go.dev/tour", Snippet: "A tour of Go"},
			}, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{Query: "golang"})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "1. [Go Programming](https://go.dev)")
	assert.Contains(t, text, "The Go language")
	assert.Contains(t, text, "2. [Go Tutorial](https://go.dev/tour)")
	assert.Contains(t, text, "A tour of Go")
}

func TestWebSearchTool_Execute_EmptyQuery(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return nil, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{Query: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "query is required")
}

func TestWebSearchTool_Execute_NilBackend(t *testing.T) {
	tool := &WebSearchTool{Search: nil}

	result, err := tool.Execute(context.Background(), WebSearchInput{Query: "test"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "search backend not configured")
}

func TestWebSearchTool_Execute_SearchError(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return nil, fmt.Errorf("rate limited")
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{Query: "test"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "search failed")
	assert.Contains(t, extractText(result), "rate limited")
}

func TestWebSearchTool_Execute_NoResults(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return []SearchResult{}, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{Query: "xyznonexistent"})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "No results found.")
}

func TestWebSearchTool_Execute_AllowedDomains(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Go", URL: "https://go.dev/doc", Snippet: "Go docs"},
				{Title: "Python", URL: "https://python.org/doc", Snippet: "Python docs"},
				{Title: "Go Blog", URL: "https://go.dev/blog", Snippet: "Go blog"},
			}, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{
		Query:          "programming",
		AllowedDomains: []string{"go.dev"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "go.dev")
	assert.NotContains(t, text, "python.org")
}

func TestWebSearchTool_Execute_BlockedDomains(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Go", URL: "https://go.dev/doc", Snippet: "Go docs"},
				{Title: "Spam", URL: "https://spam.com/page", Snippet: "Spam content"},
			}, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{
		Query:          "programming",
		BlockedDomains: []string{"spam.com"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "go.dev")
	assert.NotContains(t, text, "spam.com")
}

func TestWebSearchTool_Execute_AllFilteredOut(t *testing.T) {
	tool := &WebSearchTool{
		Search: func(_ context.Context, query string) ([]SearchResult, error) {
			return []SearchResult{
				{Title: "Spam", URL: "https://spam.com/page", Snippet: "Spam"},
			}, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebSearchInput{
		Query:          "test",
		BlockedDomains: []string{"spam.com"},
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)
	assert.Contains(t, extractText(result), "No results found.")
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com/path", "example.com"},
		{"http://example.com/path", "example.com"},
		{"https://sub.example.com/path?q=1", "sub.example.com"},
		{"https://example.com", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractDomain(tt.url))
		})
	}
}

func TestFilterResults(t *testing.T) {
	results := []SearchResult{
		{Title: "A", URL: "https://a.com/1"},
		{Title: "B", URL: "https://b.com/2"},
		{Title: "C", URL: "https://c.com/3"},
	}

	t.Run("no filters", func(t *testing.T) {
		filtered := filterResults(results, nil, nil)
		assert.Len(t, filtered, 3)
	})

	t.Run("allow only a.com", func(t *testing.T) {
		filtered := filterResults(results, []string{"a.com"}, nil)
		assert.Len(t, filtered, 1)
		assert.Equal(t, "A", filtered[0].Title)
	})

	t.Run("block b.com", func(t *testing.T) {
		filtered := filterResults(results, nil, []string{"b.com"})
		assert.Len(t, filtered, 2)
	})

	t.Run("case insensitive", func(t *testing.T) {
		filtered := filterResults(results, []string{"A.COM"}, nil)
		assert.Len(t, filtered, 1)
	})
}
