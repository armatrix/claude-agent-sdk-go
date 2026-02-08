package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/armatrix/claude-agent-sdk-go/mcp"
)

// mockMCPTransport implements mcp.Transport for testing the resource tools.
type mockMCPTransport struct {
	resources []mcp.Resource
	readFn    func(ctx context.Context, uri string) (string, error)
	connected bool
}

func (m *mockMCPTransport) Connect(_ context.Context) error {
	m.connected = true
	return nil
}

func (m *mockMCPTransport) ListTools(_ context.Context) ([]mcp.ToolInfo, error) {
	return nil, nil
}

func (m *mockMCPTransport) CallTool(_ context.Context, _ string, _ map[string]any) (string, error) {
	return "", nil
}

func (m *mockMCPTransport) ListResources(_ context.Context) ([]mcp.Resource, error) {
	if !m.connected {
		return nil, mcp.ErrNotConnected
	}
	return m.resources, nil
}

func (m *mockMCPTransport) ReadResource(ctx context.Context, uri string) (string, error) {
	if !m.connected {
		return "", mcp.ErrNotConnected
	}
	if m.readFn != nil {
		return m.readFn(ctx, uri)
	}
	return "default content", nil
}

func (m *mockMCPTransport) Close() error {
	m.connected = false
	return nil
}

func newTestManager(t *testing.T, serverName string, transport mcp.Transport) *mcp.Manager {
	t.Helper()
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{
		serverName: transport,
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))
	return mgr
}

func TestListMcpResourcesTool_Name(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewListMcpResourcesTool(mgr)
	assert.Equal(t, "ListMcpResources", tool.Name())
}

func TestListMcpResourcesTool_EmptyServerName(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewListMcpResourcesTool(mgr)

	result, err := tool.Execute(context.Background(), ListMcpResourcesInput{})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestListMcpResourcesTool_NoResources(t *testing.T) {
	mock := &mockMCPTransport{}
	mgr := newTestManager(t, "empty", mock)
	tool := NewListMcpResourcesTool(mgr)

	result, err := tool.Execute(context.Background(), ListMcpResourcesInput{
		ServerName: "empty",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Equal(t, "No resources available.", text)
}

func TestListMcpResourcesTool_WithResources(t *testing.T) {
	mock := &mockMCPTransport{
		resources: []mcp.Resource{
			{URI: "file:///a.txt", Name: "a.txt", Description: "File A", MIMEType: "text/plain"},
			{URI: "db://users", Name: "users", Description: "Users table"},
		},
	}
	mgr := newTestManager(t, "data", mock)
	tool := NewListMcpResourcesTool(mgr)

	result, err := tool.Execute(context.Background(), ListMcpResourcesInput{
		ServerName: "data",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Contains(t, text, "a.txt")
	assert.Contains(t, text, "file:///a.txt")
	assert.Contains(t, text, "File A")
	assert.Contains(t, text, "text/plain")
	assert.Contains(t, text, "users")
	assert.Contains(t, text, "db://users")
}

func TestListMcpResourcesTool_ServerNotFound(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewListMcpResourcesTool(mgr)

	result, err := tool.Execute(context.Background(), ListMcpResourcesInput{
		ServerName: "nonexistent",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "server not found")
}

func TestReadMcpResourceTool_Name(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewReadMcpResourceTool(mgr)
	assert.Equal(t, "ReadMcpResource", tool.Name())
}

func TestReadMcpResourceTool_EmptyServerName(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewReadMcpResourceTool(mgr)

	result, err := tool.Execute(context.Background(), ReadMcpResourceInput{URI: "file:///test"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestReadMcpResourceTool_EmptyURI(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewReadMcpResourceTool(mgr)

	result, err := tool.Execute(context.Background(), ReadMcpResourceInput{ServerName: "srv"})
	require.NoError(t, err)
	assert.True(t, result.IsError)
}

func TestReadMcpResourceTool_Success(t *testing.T) {
	mock := &mockMCPTransport{
		readFn: func(_ context.Context, uri string) (string, error) {
			return "content of " + uri, nil
		},
	}
	mgr := newTestManager(t, "docs", mock)
	tool := NewReadMcpResourceTool(mgr)

	result, err := tool.Execute(context.Background(), ReadMcpResourceInput{
		ServerName: "docs",
		URI:        "file:///readme.md",
	})
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := extractText(result)
	assert.Equal(t, "content of file:///readme.md", text)
}

func TestReadMcpResourceTool_ServerNotFound(t *testing.T) {
	mgr := mcp.NewManagerWithTransports(map[string]mcp.Transport{})
	tool := NewReadMcpResourceTool(mgr)

	result, err := tool.Execute(context.Background(), ReadMcpResourceInput{
		ServerName: "missing",
		URI:        "file:///test",
	})
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, extractText(result), "server not found")
}

// Verify that the tools satisfy the agent.Tool interface via type assertions
// already done in mcpresource.go (var _ agent.Tool[...] = ...) — these tests
// exercise the Execute path end-to-end.

func TestMcpResourceTools_JSONSchema(t *testing.T) {
	// Verify the input types produce valid JSON when marshaled,
	// which is a prerequisite for schema generation.
	listInput := ListMcpResourcesInput{ServerName: "test"}
	data, err := json.Marshal(listInput)
	require.NoError(t, err)
	assert.Contains(t, string(data), "test")

	readInput := ReadMcpResourceInput{ServerName: "test", URI: "file:///x"}
	data, err = json.Marshal(readInput)
	require.NoError(t, err)
	assert.Contains(t, string(data), "file:///x")
}
