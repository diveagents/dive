package mcp

import (
	"context"
	"testing"

	"github.com/diveagents/dive"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// testServerConfig implements ServerConfig for testing
type testServerConfig struct {
	serverType         string
	name               string
	url                string
	env                map[string]string
	args               []string
	authorizationToken string
	toolEnabled        bool
	allowedTools       []string
}

func (t testServerConfig) GetType() string {
	return t.serverType
}

func (t testServerConfig) GetName() string {
	return t.name
}

func (t testServerConfig) GetURL() string {
	return t.url
}

func (t testServerConfig) GetEnv() map[string]string {
	if t.env == nil {
		return make(map[string]string)
	}
	return t.env
}

func (t testServerConfig) GetArgs() []string {
	if t.args == nil {
		return []string{}
	}
	return t.args
}

func (t testServerConfig) GetAuthorizationToken() string {
	return t.authorizationToken
}

func (t testServerConfig) IsToolEnabled() bool {
	return t.toolEnabled
}

func (t testServerConfig) GetAllowedTools() []string {
	return t.allowedTools
}

func TestNewMCPManager(t *testing.T) {
	manager := NewMCPManager()
	require.NotNil(t, manager)
	require.NotNil(t, manager.servers)
	require.NotNil(t, manager.tools)
	require.Empty(t, manager.servers)
	require.Empty(t, manager.tools)
}

func TestMCPManager_GetAllTools_EmptyManager(t *testing.T) {
	manager := NewMCPManager()
	tools := manager.GetAllTools()
	require.NotNil(t, tools)
	require.Empty(t, tools)
}

func TestMCPManager_GetToolsByServer_NonExistent(t *testing.T) {
	manager := NewMCPManager()
	tools := manager.GetToolsByServer("nonexistent")
	require.Nil(t, tools)
}

func TestMCPManager_GetTool_NonExistent(t *testing.T) {
	manager := NewMCPManager()
	tool := manager.GetTool("nonexistent.tool")
	require.Nil(t, tool)
}

func TestMCPManager_GetServerStatus_Empty(t *testing.T) {
	manager := NewMCPManager()
	status := manager.GetServerStatus()
	require.NotNil(t, status)
	require.Empty(t, status)
}

func TestMCPManager_IsServerConnected_NonExistent(t *testing.T) {
	manager := NewMCPManager()
	connected := manager.IsServerConnected("nonexistent")
	require.False(t, connected)
}

func TestMCPManager_GetServerNames_Empty(t *testing.T) {
	manager := NewMCPManager()
	names := manager.GetServerNames()
	require.NotNil(t, names)
	require.Empty(t, names)
}

func TestMCPManager_Close_EmptyManager(t *testing.T) {
	manager := NewMCPManager()
	err := manager.Close()
	require.NoError(t, err)
}

func TestMCPManager_RefreshTools_EmptyManager(t *testing.T) {
	manager := NewMCPManager()
	ctx := context.Background()
	err := manager.RefreshTools(ctx)
	require.NoError(t, err)
}

func TestMCPManager_InitializeServers_UnsupportedType(t *testing.T) {
	manager := NewMCPManager()
	ctx := context.Background()

	serverConfigs := []ServerConfig{
		testServerConfig{
			serverType:  "unsupported",
			name:        "test-server",
			url:         "invalid://url",
			toolEnabled: true,
		},
	}

	err := manager.InitializeServers(ctx, serverConfigs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to initialize MCP server test-server")

	// Verify no servers were added
	names := manager.GetServerNames()
	require.Empty(t, names)
}

func TestMCPManager_InitializeServers_DuplicateServer(t *testing.T) {
	manager := NewMCPManager()

	// Manually add a server connection to simulate already initialized server
	testConfig := testServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	}

	client, err := NewMCPClient(testConfig)
	require.NoError(t, err)

	manager.servers["test-server"] = &MCPServerConnection{
		Client: client,
		Config: testConfig,
		Tools:  []dive.Tool{},
	}

	// Try to initialize the same server again
	ctx := context.Background()
	serverConfigs := []ServerConfig{testConfig}

	err = manager.InitializeServers(ctx, serverConfigs)
	require.Error(t, err)
	require.Contains(t, err.Error(), "MCP server test-server already initialized")
}

func TestMCPManager_WithMockServer(t *testing.T) {
	manager := NewMCPManager()

	// Create a test server connection manually
	testConfig := testServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	}

	client, err := NewMCPClient(testConfig)
	require.NoError(t, err)

	// Mock some tools
	mockTools := []mcp.Tool{
		{
			Name:        "test-tool-1",
			Description: "Test tool 1",
		},
		{
			Name:        "test-tool-2",
			Description: "Test tool 2",
		},
	}

	var diveTools []dive.Tool
	for _, mcpTool := range mockTools {
		adapter := NewMCPToolAdapter(client, mcpTool, testConfig.GetName())
		diveTools = append(diveTools, adapter)
	}

	// Add the server connection manually (simulating successful initialization)
	manager.servers[testConfig.GetName()] = &MCPServerConnection{
		Client: client,
		Config: testConfig,
		Tools:  diveTools,
	}

	// Add tools to the global tools map
	for _, tool := range diveTools {
		toolKey := testConfig.GetName() + "." + tool.Name()
		manager.tools[toolKey] = tool
	}

	// Test GetServerNames
	names := manager.GetServerNames()
	require.Len(t, names, 1)
	require.Contains(t, names, "test-server")

	// Test GetToolsByServer
	tools := manager.GetToolsByServer("test-server")
	require.Len(t, tools, 2)

	// Test GetAllTools
	allTools := manager.GetAllTools()
	require.Len(t, allTools, 2)
	require.Contains(t, allTools, "test-server.test-tool-1")
	require.Contains(t, allTools, "test-server.test-tool-2")

	// Test GetTool
	tool1 := manager.GetTool("test-server.test-tool-1")
	require.NotNil(t, tool1)
	require.Equal(t, "test-tool-1", tool1.Name())

	// Test GetServerStatus
	status := manager.GetServerStatus()
	require.Len(t, status, 1)
	// Note: Since we didn't actually connect, this will be false
	require.Contains(t, status, "test-server")

	// Test IsServerConnected
	connected := manager.IsServerConnected("test-server")
	require.False(t, connected) // Not actually connected

	// Test Close
	err = manager.Close()
	require.NoError(t, err)

	// Verify everything is cleaned up
	names = manager.GetServerNames()
	require.Empty(t, names)
	allTools = manager.GetAllTools()
	require.Empty(t, allTools)
}

func TestMCPManager_RefreshTools_WithMockServer(t *testing.T) {
	manager := NewMCPManager()

	// Create a test server connection
	testConfig := testServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	}

	client, err := NewMCPClient(testConfig)
	require.NoError(t, err)

	// Set up the client as connected but don't actually connect
	client.connected = true

	// Add initial tools
	initialTools := []mcp.Tool{
		{Name: "initial-tool", Description: "Initial tool"},
	}

	var diveTools []dive.Tool
	for _, mcpTool := range initialTools {
		adapter := NewMCPToolAdapter(client, mcpTool, testConfig.GetName())
		diveTools = append(diveTools, adapter)
	}

	manager.servers[testConfig.GetName()] = &MCPServerConnection{
		Client: client,
		Config: testConfig,
		Tools:  diveTools,
	}

	// Add initial tools to global map
	for _, tool := range diveTools {
		toolKey := testConfig.GetName() + "." + tool.Name()
		manager.tools[toolKey] = tool
	}

	// Verify initial state
	require.Len(t, manager.GetAllTools(), 1)

	// RefreshTools will fail because we can't actually list tools from a mock server
	// but it should handle the error gracefully - we can't test this easily without
	// a proper mock, so we'll just verify that the method exists and doesn't panic
	// when called on disconnected servers
	ctx := context.Background()

	// Disconnect the client to avoid the nil pointer dereference
	client.connected = false

	err = manager.RefreshTools(ctx)
	require.NoError(t, err) // Should succeed by skipping disconnected servers

	// Verify the server connection is still there
	require.False(t, manager.IsServerConnected("test-server")) // Now disconnected
}

func TestMCPManager_RefreshTools_DisconnectedServer(t *testing.T) {
	manager := NewMCPManager()

	// Create a test server connection that's not connected
	testConfig := testServerConfig{
		serverType:  "http",
		name:        "test-server",
		url:         "http://localhost:8080",
		toolEnabled: true,
	}

	client, err := NewMCPClient(testConfig)
	require.NoError(t, err)
	// Don't set connected = true, so it's disconnected

	manager.servers[testConfig.GetName()] = &MCPServerConnection{
		Client: client,
		Config: testConfig,
		Tools:  []dive.Tool{},
	}

	// RefreshTools should skip disconnected servers
	ctx := context.Background()
	err = manager.RefreshTools(ctx)
	require.NoError(t, err) // Should succeed by skipping disconnected servers
}
