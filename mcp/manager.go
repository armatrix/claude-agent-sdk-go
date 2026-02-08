package mcp

import (
	"context"
	"sync"
)

// serverConn represents an active connection to a single MCP server.
type serverConn struct {
	name   string
	config ServerConfig
	ctx    context.Context
	cancel context.CancelFunc
	// client and tools will be populated during implementation
}

// Manager manages connections to multiple MCP servers.
type Manager struct {
	servers map[string]*serverConn
	mu      sync.RWMutex
}

// NewManager creates a Manager from the given server configurations.
func NewManager(configs map[string]ServerConfig) *Manager {
	return &Manager{
		servers: make(map[string]*serverConn),
	}
}

// Connect establishes connections to all configured servers.
// It discovers remote tools and prepares them for bridging.
func (m *Manager) Connect(ctx context.Context) error {
	// TODO: implement — connect to each server, ListTools, bridge
	_ = ctx
	return nil
}

// Close gracefully disconnects from all servers.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sc := range m.servers {
		if sc.cancel != nil {
			sc.cancel()
		}
	}
	m.servers = make(map[string]*serverConn)
	return nil
}

// ServerNames returns the names of all configured servers.
func (m *Manager) ServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}
