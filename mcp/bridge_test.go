package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridgeToolName(t *testing.T) {
	tests := []struct {
		server string
		tool   string
		want   string
	}{
		{"context7", "query-docs", "mcp__context7__query-docs"},
		{"playwright", "browser_click", "mcp__playwright__browser_click"},
		{"my-server", "my-tool", "mcp__my-server__my-tool"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := BridgeToolName(tt.server, tt.tool)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseBridgedName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantServer string
		wantTool   string
		wantErr    bool
	}{
		{
			name:       "valid simple",
			input:      "mcp__server1__tool1",
			wantServer: "server1",
			wantTool:   "tool1",
		},
		{
			name:       "valid with hyphens",
			input:      "mcp__my-server__my-tool",
			wantServer: "my-server",
			wantTool:   "my-tool",
		},
		{
			name:       "valid with underscores in tool",
			input:      "mcp__playwright__browser_click",
			wantServer: "playwright",
			wantTool:   "browser_click",
		},
		{
			name:       "tool name with double underscore",
			input:      "mcp__server__tool__with__extra",
			wantServer: "server",
			wantTool:   "tool__with__extra",
		},
		{
			name:    "missing prefix",
			input:   "server__tool",
			wantErr: true,
		},
		{
			name:    "no separator after server",
			input:   "mcp__servertool",
			wantErr: true,
		},
		{
			name:    "empty server name",
			input:   "mcp____tool",
			wantErr: true,
		},
		{
			name:    "empty tool name",
			input:   "mcp__server__",
			wantErr: true,
		},
		{
			name:    "completely invalid",
			input:   "not_an_mcp_tool",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, tool, err := ParseBridgedName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrToolNotFound)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantServer, server)
			assert.Equal(t, tt.wantTool, tool)
		})
	}
}

func TestBridgeToolName_Roundtrip(t *testing.T) {
	server := "context7"
	tool := "query-docs"

	fullName := BridgeToolName(server, tool)
	gotServer, gotTool, err := ParseBridgedName(fullName)

	require.NoError(t, err)
	assert.Equal(t, server, gotServer)
	assert.Equal(t, tool, gotTool)
}
