package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport is a Transport stub that returns canned data for testing.
type mockTransport struct {
	tools     []ToolInfo
	resources []Resource
	callFn    func(ctx context.Context, name string, args map[string]any) (string, error)
	readFn    func(ctx context.Context, uri string) (string, error)
	connected bool
	closeCh   chan struct{}
}

func newMockTransport(tools []ToolInfo, resources []Resource) *mockTransport {
	return &mockTransport{
		tools:     tools,
		resources: resources,
		closeCh:   make(chan struct{}, 1),
	}
}

func (m *mockTransport) Connect(_ context.Context) error {
	m.connected = true
	return nil
}

func (m *mockTransport) ListTools(_ context.Context) ([]ToolInfo, error) {
	if !m.connected {
		return nil, ErrNotConnected
	}
	return m.tools, nil
}

func (m *mockTransport) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	if !m.connected {
		return "", ErrNotConnected
	}
	if m.callFn != nil {
		return m.callFn(ctx, name, args)
	}
	return "mock result", nil
}

func (m *mockTransport) ListResources(_ context.Context) ([]Resource, error) {
	if !m.connected {
		return nil, ErrNotConnected
	}
	return m.resources, nil
}

func (m *mockTransport) ReadResource(ctx context.Context, uri string) (string, error) {
	if !m.connected {
		return "", ErrNotConnected
	}
	if m.readFn != nil {
		return m.readFn(ctx, uri)
	}
	return "mock resource content", nil
}

func (m *mockTransport) Close() error {
	m.connected = false
	m.closeCh <- struct{}{}
	return nil
}

func TestNewManager(t *testing.T) {
	configs := map[string]ServerConfig{
		"server1": {Command: "echo", Transport: TransportStdio},
		"server2": {URL: "http://localhost:8080", Transport: TransportSSE},
	}
	mgr := NewManager(configs)
	require.NotNil(t, mgr)
	assert.Len(t, mgr.configs, 2)
	assert.Empty(t, mgr.servers, "no servers connected yet")
}

func TestNewManager_CopiesConfigs(t *testing.T) {
	configs := map[string]ServerConfig{
		"s1": {Command: "echo"},
	}
	mgr := NewManager(configs)

	// Mutate original — should not affect manager.
	configs["s2"] = ServerConfig{Command: "cat"}
	assert.Len(t, mgr.configs, 1)
}

func TestNewManagerWithTransports(t *testing.T) {
	mock := newMockTransport(nil, nil)
	mgr := NewManagerWithTransports(map[string]Transport{
		"test": mock,
	})
	require.NotNil(t, mgr)
	names := mgr.ServerNames()
	assert.Contains(t, names, "test")
}

func TestManager_ConnectWithTransports(t *testing.T) {
	tools := []ToolInfo{
		{Name: "search", Description: "Search files", InputSchema: json.RawMessage(`{"type":"object"}`)},
		{Name: "read", Description: "Read file", InputSchema: json.RawMessage(`{"type":"object"}`)},
	}
	mock := newMockTransport(tools, nil)
	mgr := NewManagerWithTransports(map[string]Transport{
		"filesystem": mock,
	})

	err := mgr.ConnectWithTransports(context.Background())
	require.NoError(t, err)

	assert.True(t, mock.connected)

	bridged := mgr.BridgedTools()
	assert.Len(t, bridged, 2)

	// Verify naming convention.
	nameSet := make(map[string]bool)
	for _, bt := range bridged {
		nameSet[bt.FullName] = true
		assert.Equal(t, "filesystem", bt.ServerName)
	}
	assert.True(t, nameSet["mcp__filesystem__search"])
	assert.True(t, nameSet["mcp__filesystem__read"])
}

func TestManager_BridgedTools_MultipleServers(t *testing.T) {
	tools1 := []ToolInfo{
		{Name: "tool_a", Description: "Tool A"},
	}
	tools2 := []ToolInfo{
		{Name: "tool_b", Description: "Tool B"},
		{Name: "tool_c", Description: "Tool C"},
	}

	mgr := NewManagerWithTransports(map[string]Transport{
		"server1": newMockTransport(tools1, nil),
		"server2": newMockTransport(tools2, nil),
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	bridged := mgr.BridgedTools()
	assert.Len(t, bridged, 3)
}

func TestManager_BridgedTools_Empty(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{
		"empty": newMockTransport(nil, nil),
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	bridged := mgr.BridgedTools()
	assert.Empty(t, bridged)
}

func TestManager_CallTool(t *testing.T) {
	tools := []ToolInfo{
		{Name: "greet", Description: "Greet someone"},
	}
	mock := newMockTransport(tools, nil)
	mock.callFn = func(_ context.Context, name string, args map[string]any) (string, error) {
		return "hello " + args["name"].(string), nil
	}

	mgr := NewManagerWithTransports(map[string]Transport{
		"greeter": mock,
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	result, err := mgr.CallTool(context.Background(), "mcp__greeter__greet", map[string]any{"name": "world"})
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestManager_CallTool_ServerNotFound(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	_, err := mgr.CallTool(context.Background(), "mcp__unknown__tool", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestManager_CallTool_ToolNotFound(t *testing.T) {
	tools := []ToolInfo{
		{Name: "existing", Description: "exists"},
	}
	mgr := NewManagerWithTransports(map[string]Transport{
		"srv": newMockTransport(tools, nil),
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	_, err := mgr.CallTool(context.Background(), "mcp__srv__nonexistent", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrToolNotFound)
}

func TestManager_CallTool_InvalidFormat(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{})

	_, err := mgr.CallTool(context.Background(), "invalid_name", nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrToolNotFound)
}

func TestManager_CallToolRaw(t *testing.T) {
	tools := []ToolInfo{
		{Name: "echo", Description: "Echo input"},
	}
	mock := newMockTransport(tools, nil)
	mock.callFn = func(_ context.Context, name string, args map[string]any) (string, error) {
		return args["msg"].(string), nil
	}

	mgr := NewManagerWithTransports(map[string]Transport{
		"srv": mock,
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	input := json.RawMessage(`{"msg":"hello"}`)
	result, err := mgr.CallToolRaw(context.Background(), "mcp__srv__echo", input)
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestManager_CallToolRaw_InvalidJSON(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{
		"srv": newMockTransport(nil, nil),
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	_, err := mgr.CallToolRaw(context.Background(), "mcp__srv__tool", json.RawMessage(`{invalid`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tool input")
}

func TestManager_ListResources(t *testing.T) {
	resources := []Resource{
		{URI: "file:///test.txt", Name: "test.txt", Description: "A test file", MIMEType: "text/plain"},
	}
	mgr := NewManagerWithTransports(map[string]Transport{
		"files": newMockTransport(nil, resources),
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	result, err := mgr.ListResources(context.Background(), "files")
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "file:///test.txt", result[0].URI)
	assert.Equal(t, "test.txt", result[0].Name)
}

func TestManager_ListResources_ServerNotFound(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{})

	_, err := mgr.ListResources(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestManager_ReadResource(t *testing.T) {
	mock := newMockTransport(nil, nil)
	mock.readFn = func(_ context.Context, uri string) (string, error) {
		return "content of " + uri, nil
	}
	mgr := NewManagerWithTransports(map[string]Transport{
		"docs": mock,
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))

	result, err := mgr.ReadResource(context.Background(), "docs", "file:///readme.md")
	require.NoError(t, err)
	assert.Equal(t, "content of file:///readme.md", result)
}

func TestManager_ReadResource_ServerNotFound(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{})

	_, err := mgr.ReadResource(context.Background(), "missing", "file:///test")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrServerNotFound)
}

func TestManager_Close(t *testing.T) {
	mock := newMockTransport(nil, nil)
	mgr := NewManagerWithTransports(map[string]Transport{
		"srv": mock,
	})
	require.NoError(t, mgr.ConnectWithTransports(context.Background()))
	assert.True(t, mock.connected)

	err := mgr.Close()
	require.NoError(t, err)

	// Verify transport was closed.
	assert.False(t, mock.connected)

	// Verify servers map is cleared.
	assert.Empty(t, mgr.ServerNames())
}

func TestManager_ServerNames(t *testing.T) {
	mgr := NewManagerWithTransports(map[string]Transport{
		"alpha": newMockTransport(nil, nil),
		"beta":  newMockTransport(nil, nil),
	})

	names := mgr.ServerNames()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "alpha")
	assert.Contains(t, names, "beta")
}
