package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// serverConn represents an active connection to a single MCP server.
type serverConn struct {
	name      string
	config    ServerConfig
	transport Transport
	tools     []ToolInfo
	ctx       context.Context
	cancel    context.CancelFunc
}

// Manager manages connections to multiple MCP servers.
type Manager struct {
	configs map[string]ServerConfig
	servers map[string]*serverConn
	mu      sync.RWMutex
}

// NewManager creates a Manager from the given server configurations.
// Call Connect to establish connections.
func NewManager(configs map[string]ServerConfig) *Manager {
	cfgs := make(map[string]ServerConfig, len(configs))
	for k, v := range configs {
		cfgs[k] = v
	}
	return &Manager{
		configs: cfgs,
		servers: make(map[string]*serverConn),
	}
}

// NewManagerWithTransports creates a Manager with pre-built transports.
// This is primarily useful for testing with mock transports.
func NewManagerWithTransports(transports map[string]Transport) *Manager {
	m := &Manager{
		configs: make(map[string]ServerConfig),
		servers: make(map[string]*serverConn),
	}
	for name, t := range transports {
		m.servers[name] = &serverConn{
			name:      name,
			transport: t,
		}
	}
	return m
}

// Connect establishes connections to all configured servers.
// It creates transports, connects each one, discovers tools, and stores
// them for bridging. Errors from individual servers are collected and
// returned as a combined error; other servers continue connecting.
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string

	for name, cfg := range m.configs {
		transport, err := NewTransport(cfg)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", name, err.Error()))
			continue
		}

		sctx, cancel := context.WithCancel(ctx)

		if err := transport.Connect(sctx); err != nil {
			cancel()
			errs = append(errs, fmt.Sprintf("%s: %s", name, err.Error()))
			continue
		}

		tools, err := transport.ListTools(sctx)
		if err != nil {
			// Non-fatal: server connected but tools listing failed.
			// Store the connection anyway; tools may become available later.
			tools = nil
		}

		m.servers[name] = &serverConn{
			name:      name,
			config:    cfg,
			transport: transport,
			tools:     tools,
			ctx:       sctx,
			cancel:    cancel,
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("mcp: connect errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ConnectWithTransports connects pre-injected transports (set via
// NewManagerWithTransports). It calls Connect and ListTools on each.
func (m *Manager) ConnectWithTransports(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string

	for name, sc := range m.servers {
		sctx, cancel := context.WithCancel(ctx)

		if err := sc.transport.Connect(sctx); err != nil {
			cancel()
			errs = append(errs, fmt.Sprintf("%s: %s", name, err.Error()))
			continue
		}

		tools, err := sc.transport.ListTools(sctx)
		if err != nil {
			tools = nil
		}

		sc.tools = tools
		sc.ctx = sctx
		sc.cancel = cancel
	}

	if len(errs) > 0 {
		return fmt.Errorf("mcp: connect errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Close gracefully disconnects from all servers.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for name, sc := range m.servers {
		if sc.cancel != nil {
			sc.cancel()
		}
		if sc.transport != nil {
			if err := sc.transport.Close(); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %s", name, err.Error()))
			}
		}
	}
	m.servers = make(map[string]*serverConn)

	if len(errs) > 0 {
		return fmt.Errorf("mcp: close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ServerNames returns the names of all connected servers.
func (m *Manager) ServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// BridgedTools returns all tools discovered from connected servers,
// adapted with namespaced names for the agent's ToolRegistry.
func (m *Manager) BridgedTools() []*BridgedTool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*BridgedTool
	for serverName, sc := range m.servers {
		for _, tool := range sc.tools {
			result = append(result, &BridgedTool{
				ServerName:  serverName,
				ToolName:    tool.Name,
				FullName:    BridgeToolName(serverName, tool.Name),
				Description: tool.Description,
				InputSchema: tool.InputSchema,
			})
		}
	}
	return result
}

// CallTool invokes a tool on a connected server. The fullName must be a
// namespaced tool name in the format "mcp__{server}__{tool}".
func (m *Manager) CallTool(ctx context.Context, fullName string, args map[string]any) (string, error) {
	serverName, toolName, err := ParseBridgedName(fullName)
	if err != nil {
		return "", err
	}

	m.mu.RLock()
	sc, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
	}

	// Verify the tool exists on this server.
	found := false
	for _, t := range sc.tools {
		if t.Name == toolName {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("%w: %s on server %s", ErrToolNotFound, toolName, serverName)
	}

	return sc.transport.CallTool(ctx, toolName, args)
}

// CallToolRaw is like CallTool but accepts raw JSON input and parses it
// into a map before delegating. This is useful for integration with the
// agent's ToolRegistry which passes json.RawMessage.
func (m *Manager) CallToolRaw(ctx context.Context, fullName string, input json.RawMessage) (string, error) {
	var args map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("mcp: invalid tool input: %w", err)
		}
	}
	return m.CallTool(ctx, fullName, args)
}

// ListResources lists resources from a specific connected server.
func (m *Manager) ListResources(ctx context.Context, serverName string) ([]Resource, error) {
	m.mu.RLock()
	sc, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
	}

	return sc.transport.ListResources(ctx)
}

// ReadResource reads a resource by URI from a specific connected server.
func (m *Manager) ReadResource(ctx context.Context, serverName string, uri string) (string, error) {
	m.mu.RLock()
	sc, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("%w: %s", ErrServerNotFound, serverName)
	}

	return sc.transport.ReadResource(ctx, uri)
}

// ParseBridgedName extracts the server name and tool name from a bridged
// tool name. The expected format is "mcp__{server}__{tool}".
func ParseBridgedName(fullName string) (serverName, toolName string, err error) {
	const prefix = "mcp__"
	if !strings.HasPrefix(fullName, prefix) {
		return "", "", fmt.Errorf("%w: invalid bridged name format: %s", ErrToolNotFound, fullName)
	}

	rest := fullName[len(prefix):]
	idx := strings.Index(rest, "__")
	if idx < 0 {
		return "", "", fmt.Errorf("%w: invalid bridged name format: %s", ErrToolNotFound, fullName)
	}

	serverName = rest[:idx]
	toolName = rest[idx+2:]

	if serverName == "" || toolName == "" {
		return "", "", fmt.Errorf("%w: invalid bridged name format: %s", ErrToolNotFound, fullName)
	}

	return serverName, toolName, nil
}
