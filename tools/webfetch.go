package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	agent "github.com/armatrix/claude-agent-sdk-go"
)

const (
	maxFetchBytes   = 512_000 // 512KB max response
	fetchTimeoutSec = 30
)

// WebFetchInput defines the input for the WebFetch tool.
type WebFetchInput struct {
	URL    string `json:"url" jsonschema:"required,description=The URL to fetch content from"`
	Prompt string `json:"prompt" jsonschema:"required,description=What information to extract from the page"`
}

// FetchFunc is an injectable HTTP fetch function for testing.
type FetchFunc func(ctx context.Context, url string) (string, error)

// WebFetchTool fetches content from a URL.
type WebFetchTool struct {
	Fetcher FetchFunc // Injectable for testing; nil uses default HTTP client
}

var _ agent.Tool[WebFetchInput] = (*WebFetchTool)(nil)

func (t *WebFetchTool) Name() string        { return "WebFetch" }
func (t *WebFetchTool) Description() string  { return "Fetch content from a URL and process it" }

func (t *WebFetchTool) Execute(ctx context.Context, input WebFetchInput) (*agent.ToolResult, error) {
	if input.URL == "" {
		return agent.ErrorResult("url is required"), nil
	}

	// Upgrade HTTP to HTTPS
	url := input.URL
	if strings.HasPrefix(url, "http://") {
		url = "https://" + url[7:]
	}

	var content string
	var err error

	if t.Fetcher != nil {
		content, err = t.Fetcher(ctx, url)
	} else {
		content, err = defaultFetch(ctx, url)
	}

	if err != nil {
		return agent.ErrorResult(fmt.Sprintf("fetch failed: %s", err.Error())), nil
	}

	// Basic HTML to text conversion (strip tags)
	text := stripHTMLTags(content)
	if len(text) > maxFetchBytes {
		text = text[:maxFetchBytes] + "\n... [content truncated]"
	}

	result := fmt.Sprintf("URL: %s\nPrompt: %s\n\nContent:\n%s", input.URL, input.Prompt, text)
	return agent.TextResult(result), nil
}

func defaultFetch(ctx context.Context, url string) (string, error) {
	client := &http.Client{Timeout: fetchTimeoutSec * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Claude-Agent-SDK/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// stripHTMLTags does a basic removal of HTML tags.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
