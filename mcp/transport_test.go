package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransport_Stdio(t *testing.T) {
	cfg := ServerConfig{
		Command:   "echo",
		Args:      []string{"hello"},
		Transport: TransportStdio,
	}
	transport, err := NewTransport(cfg)
	require.NoError(t, err)

	_, ok := transport.(*StdioTransport)
	assert.True(t, ok, "expected StdioTransport")
}

func TestNewTransport_SSE(t *testing.T) {
	cfg := ServerConfig{
		URL:       "http://localhost:8080",
		Transport: TransportSSE,
	}
	transport, err := NewTransport(cfg)
	require.NoError(t, err)

	_, ok := transport.(*HTTPTransport)
	assert.True(t, ok, "expected HTTPTransport")
}

func TestNewTransport_StreamableHTTP(t *testing.T) {
	cfg := ServerConfig{
		URL:       "http://localhost:9090",
		Transport: TransportStreamableHTTP,
	}
	transport, err := NewTransport(cfg)
	require.NoError(t, err)

	_, ok := transport.(*HTTPTransport)
	assert.True(t, ok, "expected HTTPTransport")
}

func TestNewTransport_DefaultStdio(t *testing.T) {
	cfg := ServerConfig{Command: "cat"}
	transport, err := NewTransport(cfg)
	require.NoError(t, err)

	_, ok := transport.(*StdioTransport)
	assert.True(t, ok, "expected StdioTransport for command-only config")
}

func TestNewTransport_DefaultHTTP(t *testing.T) {
	cfg := ServerConfig{URL: "http://example.com"}
	transport, err := NewTransport(cfg)
	require.NoError(t, err)

	_, ok := transport.(*HTTPTransport)
	assert.True(t, ok, "expected HTTPTransport for URL-only config")
}

func TestNewTransport_InvalidConfig(t *testing.T) {
	cfg := ServerConfig{} // no command, no URL
	_, err := NewTransport(cfg)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidConfig)
}

func TestStdioTransport_MissingCommand(t *testing.T) {
	_, err := NewStdioTransport(ServerConfig{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidConfig)
}

func TestStdioTransport_Connect(t *testing.T) {
	transport, err := NewStdioTransport(ServerConfig{Command: "echo"})
	require.NoError(t, err)

	err = transport.Connect(context.Background())
	require.NoError(t, err)
}

func TestStdioTransport_NotConnected(t *testing.T) {
	transport, err := NewStdioTransport(ServerConfig{Command: "echo"})
	require.NoError(t, err)

	_, err = transport.ListTools(context.Background())
	assert.ErrorIs(t, err, ErrNotConnected)

	_, err = transport.CallTool(context.Background(), "test", nil)
	assert.ErrorIs(t, err, ErrNotConnected)

	_, err = transport.ListResources(context.Background())
	assert.ErrorIs(t, err, ErrNotConnected)

	_, err = transport.ReadResource(context.Background(), "file:///test")
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestStdioTransport_Close(t *testing.T) {
	transport, err := NewStdioTransport(ServerConfig{Command: "echo"})
	require.NoError(t, err)

	require.NoError(t, transport.Connect(context.Background()))
	require.NoError(t, transport.Close())
	assert.False(t, transport.connected)
}

func TestHTTPTransport_MissingURL(t *testing.T) {
	_, err := NewHTTPTransport(ServerConfig{})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidConfig)
}

func TestHTTPTransport_Connect(t *testing.T) {
	transport, err := NewHTTPTransport(ServerConfig{URL: "http://localhost:8080"})
	require.NoError(t, err)

	err = transport.Connect(context.Background())
	require.NoError(t, err)
}

func TestHTTPTransport_NotConnected(t *testing.T) {
	transport, err := NewHTTPTransport(ServerConfig{URL: "http://localhost:8080"})
	require.NoError(t, err)

	_, err = transport.ListTools(context.Background())
	assert.ErrorIs(t, err, ErrNotConnected)

	_, err = transport.CallTool(context.Background(), "test", nil)
	assert.ErrorIs(t, err, ErrNotConnected)

	_, err = transport.ListResources(context.Background())
	assert.ErrorIs(t, err, ErrNotConnected)

	_, err = transport.ReadResource(context.Background(), "file:///test")
	assert.ErrorIs(t, err, ErrNotConnected)
}

func TestHTTPTransport_Close(t *testing.T) {
	transport, err := NewHTTPTransport(ServerConfig{URL: "http://localhost:8080"})
	require.NoError(t, err)

	require.NoError(t, transport.Connect(context.Background()))
	require.NoError(t, transport.Close())
	assert.False(t, transport.connected)
}
