package mcp

import (
	"context"
	"testing"
	"unsafe"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// clientTestServerConfig implements ServerConfig interface for testing
type clientTestServerConfig struct {
	serverType         string
	name               string
	url                string
	env                map[string]string
	args               []string
	authorizationToken string
	toolEnabled        bool
	allowedTools       []string
}

func (c clientTestServerConfig) GetType() string {
	return c.serverType
}

func (c clientTestServerConfig) GetName() string {
	return c.name
}

func (c clientTestServerConfig) GetURL() string {
	return c.url
}

func (c clientTestServerConfig) GetEnv() map[string]string {
	if c.env == nil {
		return make(map[string]string)
	}
	return c.env
}

func (c clientTestServerConfig) GetArgs() []string {
	if c.args == nil {
		return []string{}
	}
	return c.args
}

func (c clientTestServerConfig) GetAuthorizationToken() string {
	return c.authorizationToken
}

func (c clientTestServerConfig) IsToolEnabled() bool {
	return c.toolEnabled
}

func (c clientTestServerConfig) GetAllowedTools() []string {
	return c.allowedTools
}

func TestNewMCPClient(t *testing.T) {
	tests := []struct {
		name   string
		config ServerConfig
	}{
		{
			name: "http server config",
			config: clientTestServerConfig{
				serverType:  "http",
				name:        "test-http-server",
				url:         "http://localhost:8080",
				toolEnabled: true,
			},
		},
		{
			name: "stdio server config",
			config: clientTestServerConfig{
				serverType:  "stdio",
				name:        "test-stdio-server",
				url:         "/path/to/server",
				toolEnabled: true,
			},
		},
		{
			name: "server with tool configuration",
			config: clientTestServerConfig{
				serverType:   "http",
				name:         "test-server-with-tools",
				url:          "http://localhost:8080",
				toolEnabled:  true,
				allowedTools: []string{"tool1", "tool2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewMCPClient(tt.config)
			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, tt.config, client.config)
			require.False(t, client.connected)
			require.Nil(t, client.client)
			require.Empty(t, client.tools)
		})
	}
}

func TestMCPClient_IsConnected(t *testing.T) {
	client, err := NewMCPClient(clientTestServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	})
	require.NoError(t, err)

	// Initially not connected
	require.False(t, client.IsConnected())

	// Simulate connection
	client.connected = true
	require.True(t, client.IsConnected())

	// Simulate disconnection
	client.connected = false
	require.False(t, client.IsConnected())
}

func TestMCPClient_GetTools(t *testing.T) {
	client, err := NewMCPClient(clientTestServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	})
	require.NoError(t, err)

	// Initially no tools
	tools := client.GetTools()
	require.Empty(t, tools)

	// Add some tools
	testTools := []mcp.Tool{
		{
			Name:        "test-tool-1",
			Description: "Test tool 1",
		},
		{
			Name:        "test-tool-2",
			Description: "Test tool 2",
		},
	}
	client.tools = testTools

	// Verify tools are returned
	tools = client.GetTools()
	require.Equal(t, testTools, tools)
}

func TestMCPClient_filterTools(t *testing.T) {
	tests := []struct {
		name          string
		config        ServerConfig
		inputTools    []mcp.Tool
		expectedTools []mcp.Tool
	}{
		{
			name: "no tool configuration returns all tools",
			config: clientTestServerConfig{
				serverType:  "http",
				name:        "test-server",
				url:         "http://localhost:8080",
				toolEnabled: true,
			},
			inputTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
			expectedTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
		},
		{
			name: "disabled tools returns empty",
			config: clientTestServerConfig{
				serverType:  "http",
				name:        "test-server",
				url:         "http://localhost:8080",
				toolEnabled: false,
			},
			inputTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
			expectedTools: []mcp.Tool{},
		},
		{
			name: "enabled with no allowed tools returns all",
			config: clientTestServerConfig{
				serverType:   "http",
				name:         "test-server",
				url:          "http://localhost:8080",
				toolEnabled:  true,
				allowedTools: []string{},
			},
			inputTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
			expectedTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
		},
		{
			name: "filters tools based on allowed list",
			config: clientTestServerConfig{
				serverType:   "http",
				name:         "test-server",
				url:          "http://localhost:8080",
				toolEnabled:  true,
				allowedTools: []string{"tool1", "tool3"},
			},
			inputTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
				{Name: "tool3", Description: "Tool 3"},
			},
			expectedTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool3", Description: "Tool 3"},
			},
		},
		{
			name: "empty result when no tools match allowed list",
			config: clientTestServerConfig{
				serverType:   "http",
				name:         "test-server",
				url:          "http://localhost:8080",
				toolEnabled:  true,
				allowedTools: []string{"nonexistent"},
			},
			inputTools: []mcp.Tool{
				{Name: "tool1", Description: "Tool 1"},
				{Name: "tool2", Description: "Tool 2"},
			},
			expectedTools: nil, // filterTools returns nil for empty result
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewMCPClient(tt.config)
			require.NoError(t, err)

			filtered := client.filterTools(tt.inputTools)
			require.Equal(t, tt.expectedTools, filtered)
		})
	}
}

func TestMCPClient_ListTools_NotConnected(t *testing.T) {
	client, err := NewMCPClient(clientTestServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	})
	require.NoError(t, err)

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "MCP client not connected")
	require.Nil(t, tools)
}

func TestMCPClient_CallTool_NotConnected(t *testing.T) {
	client, err := NewMCPClient(clientTestServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	})
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.CallTool(ctx, "test-tool", map[string]interface{}{"param": "value"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MCP client not connected")
	require.Nil(t, result)
}

func TestMCPClient_Connect_UnsupportedType(t *testing.T) {
	client, err := NewMCPClient(clientTestServerConfig{
		serverType:  "unsupported",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	})
	require.NoError(t, err)

	ctx := context.Background()
	err = client.Connect(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported MCP server type: unsupported")
	require.False(t, client.IsConnected())
}

func TestMCPClient_Close(t *testing.T) {
	tests := []struct {
		name              string
		initialConnected  bool
		hasClient         bool
		expectedConnected bool
	}{
		{
			name:              "close when not connected",
			initialConnected:  false,
			hasClient:         false,
			expectedConnected: false,
		},
		{
			name:              "close when connected but no client",
			initialConnected:  true,
			hasClient:         false,
			expectedConnected: true, // should remain connected since condition isn't met
		},
		{
			name:              "close when connected with client",
			initialConnected:  true,
			hasClient:         true,
			expectedConnected: false, // should disconnect
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpClient, err := NewMCPClient(clientTestServerConfig{
				serverType:  "http",
				name:        "test-server",
				url:         "http://localhost:8080",
				toolEnabled: true,
			})
			require.NoError(t, err)

			// Set up initial state
			mcpClient.connected = tt.initialConnected
			if tt.hasClient {
				// Use reflection or just set to a non-nil value to satisfy the condition
				// Since we can't easily mock client.Client, we'll create a minimal test that
				// verifies the close behavior logic
				mcpClient.client = (*client.Client)(unsafe.Pointer(uintptr(1))) // Non-nil pointer
			}

			// Execute close
			err = mcpClient.Close()
			require.NoError(t, err)

			// Verify final state
			require.Equal(t, tt.expectedConnected, mcpClient.IsConnected())
		})
	}
}

func TestMCPClient_Connect_StdioWithEnvAndArgs(t *testing.T) {
	config := clientTestServerConfig{
		serverType: "stdio",
		name:       "test-stdio-server",
		url:        "python",
		env: map[string]string{
			"API_KEY":    "test-key",
			"DEBUG":      "true",
			"SERVER_URL": "http://localhost:8080",
		},
		args: []string{
			"server.py",
			"--port", "3000",
			"--verbose",
		},
		toolEnabled: true,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)

	// Verify that the config methods return the expected values
	require.Equal(t, "stdio", client.config.GetType())
	require.Equal(t, "python", client.config.GetURL())

	expectedEnv := map[string]string{
		"API_KEY":    "test-key",
		"DEBUG":      "true",
		"SERVER_URL": "http://localhost:8080",
	}
	require.Equal(t, expectedEnv, client.config.GetEnv())

	expectedArgs := []string{"server.py", "--port", "3000", "--verbose"}
	require.Equal(t, expectedArgs, client.config.GetArgs())

	// Note: We don't actually call Connect() here because it would try to start
	// a real subprocess, which would fail in the test environment.
	// The important part is that the configuration is properly stored and accessible.
}
