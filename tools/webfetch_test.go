package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebFetchTool_Name(t *testing.T) {
	tool := &WebFetchTool{}
	assert.Equal(t, "WebFetch", tool.Name())
}

func TestWebFetchTool_Description(t *testing.T) {
	tool := &WebFetchTool{}
	assert.NotEmpty(t, tool.Description())
}

func TestWebFetchTool_Execute_Success(t *testing.T) {
	tool := &WebFetchTool{
		Fetcher: func(_ context.Context, url string) (string, error) {
			return "Hello, world!", nil
		},
	}

	result, err := tool.Execute(context.Background(), WebFetchInput{
		URL:    "https://example.com",
		Prompt: "extract main content",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "URL: https://example.com")
	assert.Contains(t, text, "Prompt: extract main content")
	assert.Contains(t, text, "Hello, world!")
}

func TestWebFetchTool_Execute_EmptyURL(t *testing.T) {
	tool := &WebFetchTool{}
	result, err := tool.Execute(context.Background(), WebFetchInput{URL: ""})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "url is required")
}

func TestWebFetchTool_Execute_HTTPUpgrade(t *testing.T) {
	var receivedURL string
	tool := &WebFetchTool{
		Fetcher: func(_ context.Context, url string) (string, error) {
			receivedURL = url
			return "ok", nil
		},
	}

	_, err := tool.Execute(context.Background(), WebFetchInput{
		URL:    "http://example.com/page",
		Prompt: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", receivedURL)
}

func TestWebFetchTool_Execute_FetchError(t *testing.T) {
	tool := &WebFetchTool{
		Fetcher: func(_ context.Context, url string) (string, error) {
			return "", fmt.Errorf("connection refused")
		},
	}

	result, err := tool.Execute(context.Background(), WebFetchInput{
		URL:    "https://example.com",
		Prompt: "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "fetch failed")
	assert.Contains(t, extractText(result), "connection refused")
}

func TestWebFetchTool_Execute_ContentTruncation(t *testing.T) {
	longContent := strings.Repeat("a", maxFetchBytes+1000)
	tool := &WebFetchTool{
		Fetcher: func(_ context.Context, url string) (string, error) {
			return longContent, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebFetchInput{
		URL:    "https://example.com",
		Prompt: "test",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "[content truncated]")
}

func TestWebFetchTool_Execute_HTMLStripping(t *testing.T) {
	htmlContent := "<html><body><h1>Title</h1><p>Hello world</p></body></html>"
	tool := &WebFetchTool{
		Fetcher: func(_ context.Context, url string) (string, error) {
			return htmlContent, nil
		},
	}

	result, err := tool.Execute(context.Background(), WebFetchInput{
		URL:    "https://example.com",
		Prompt: "test",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.NotContains(t, text, "<html>")
	assert.NotContains(t, text, "<body>")
	assert.NotContains(t, text, "<h1>")
	assert.Contains(t, text, "Title")
	assert.Contains(t, text, "Hello world")
}

func TestWebFetchTool_Execute_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tool := &WebFetchTool{
		Fetcher: func(ctx context.Context, url string) (string, error) {
			return "", ctx.Err()
		},
	}

	result, err := tool.Execute(ctx, WebFetchInput{
		URL:    "https://example.com",
		Prompt: "test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "fetch failed")
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "simple tags",
			input:    "<p>hello</p>",
			expected: "hello",
		},
		{
			name:     "nested tags",
			input:    "<div><span>hello</span></div>",
			expected: "hello",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTMLTags(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebFetchTool_Execute_HTTPSNotUpgraded(t *testing.T) {
	var receivedURL string
	tool := &WebFetchTool{
		Fetcher: func(_ context.Context, url string) (string, error) {
			receivedURL = url
			return "ok", nil
		},
	}

	_, err := tool.Execute(context.Background(), WebFetchInput{
		URL:    "https://example.com/page",
		Prompt: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/page", receivedURL)
}
