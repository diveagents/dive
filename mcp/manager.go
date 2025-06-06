package mcp

import (
	"context"
	"fmt"
	"sync"

	"github.com/diveagents/dive"
)

// MCPServerConnection represents a connection to an MCP server
type MCPServerConnection struct {
	Client             *MCPClient
	Config             ServerConfig
	Tools              []dive.Tool
	ResourceRepository dive.DocumentRepository
}

// MCPManager manages multiple MCP server connections and tool discovery
type MCPManager struct {
	servers map[string]*MCPServerConnection
	tools   map[string]dive.Tool
	mutex   sync.RWMutex
}

// NewMCPManager creates a new MCP manager
func NewMCPManager() *MCPManager {
	return &MCPManager{
		servers: make(map[string]*MCPServerConnection),
		tools:   make(map[string]dive.Tool),
	}
}

// InitializeServers connects to and initializes all configured MCP servers
func (m *MCPManager) InitializeServers(ctx context.Context, serverConfigs []ServerConfig) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errors []error

	for _, serverConfig := range serverConfigs {
		if err := m.initializeServer(ctx, serverConfig); err != nil {
			errors = append(errors, fmt.Errorf("failed to initialize MCP server %s: %w", serverConfig.GetName(), err))
			continue
		}
	}

	if len(errors) > 0 {
		// Return the first error, but log all errors
		return errors[0]
	}

	return nil
}

// initializeServer initializes a single MCP server connection
func (m *MCPManager) initializeServer(ctx context.Context, serverConfig ServerConfig) error {
	// Check if server is already initialized
	if _, exists := m.servers[serverConfig.GetName()]; exists {
		return fmt.Errorf("MCP server %s already initialized", serverConfig.GetName())
	}

	// Create MCP client
	client, err := NewMCPClient(serverConfig)
	if err != nil {
		return fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Connect to the server
	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	// Discover tools
	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		client.Close() // Clean up on error
		return fmt.Errorf("failed to list tools: %w", err)
	}

	// Create tool adapters
	var tools []dive.Tool
	for _, mcpTool := range mcpTools {
		adapter := NewMCPToolAdapter(client, mcpTool, serverConfig.GetName())
		tools = append(tools, adapter)

		// Add to global tools map with server prefix to avoid name conflicts
		toolKey := fmt.Sprintf("%s.%s", serverConfig.GetName(), mcpTool.Name)
		m.tools[toolKey] = adapter
	}

	// Create resource repository if server supports resources
	var resourceRepo dive.DocumentRepository
	if client.GetServerCapabilities() != nil && client.GetServerCapabilities().Resources != nil {
		resourceRepo = NewMCPResourceRepository(client, serverConfig.GetName())
	}

	// Store the server connection
	m.servers[serverConfig.GetName()] = &MCPServerConnection{
		Client:             client,
		Config:             serverConfig,
		Tools:              tools,
		ResourceRepository: resourceRepo,
	}

	return nil
}

// GetAllTools returns all tools from all connected MCP servers
func (m *MCPManager) GetAllTools() map[string]dive.Tool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy of the tools map
	result := make(map[string]dive.Tool)
	for k, v := range m.tools {
		result[k] = v
	}
	return result
}

// GetToolsByServer returns tools from a specific MCP server
func (m *MCPManager) GetToolsByServer(serverName string) []dive.Tool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if server, exists := m.servers[serverName]; exists {
		return server.Tools
	}
	return nil
}

// GetTool returns a specific tool by name (with server prefix)
func (m *MCPManager) GetTool(toolKey string) dive.Tool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.tools[toolKey]
}

// GetServerStatus returns the connection status of all servers
func (m *MCPManager) GetServerStatus() map[string]bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	status := make(map[string]bool)
	for name, server := range m.servers {
		status[name] = server.Client.IsConnected()
	}
	return status
}

// RefreshTools refreshes the tool list for all servers
func (m *MCPManager) RefreshTools(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errors []error

	for serverName, server := range m.servers {
		if !server.Client.IsConnected() {
			continue
		}

		// Re-discover tools
		mcpTools, err := server.Client.ListTools(ctx)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to refresh tools for server %s: %w", serverName, err))
			continue
		}

		// Remove old tools for this server
		for toolKey := range m.tools {
			if len(toolKey) > len(serverName)+1 && toolKey[:len(serverName)+1] == serverName+"." {
				delete(m.tools, toolKey)
			}
		}

		// Create new tool adapters
		var tools []dive.Tool
		for _, mcpTool := range mcpTools {
			adapter := NewMCPToolAdapter(server.Client, mcpTool, serverName)
			tools = append(tools, adapter)

			// Add to global tools map
			toolKey := fmt.Sprintf("%s.%s", serverName, mcpTool.Name)
			m.tools[toolKey] = adapter
		}

		// Update server tools
		server.Tools = tools
	}

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// Close closes all MCP server connections
func (m *MCPManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errors []error

	for serverName, server := range m.servers {
		if err := server.Client.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close MCP server %s: %w", serverName, err))
		}
	}

	// Clear all data
	m.servers = make(map[string]*MCPServerConnection)
	m.tools = make(map[string]dive.Tool)

	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

// IsServerConnected checks if a specific server is connected
func (m *MCPManager) IsServerConnected(serverName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if server, exists := m.servers[serverName]; exists {
		return server.Client.IsConnected()
	}
	return false
}

// GetServerNames returns a list of all configured server names
func (m *MCPManager) GetServerNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// GetResourceRepository returns the resource repository for a specific server
func (m *MCPManager) GetResourceRepository(serverName string) dive.DocumentRepository {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if server, exists := m.servers[serverName]; exists {
		return server.ResourceRepository
	}
	return nil
}

// GetAllResourceRepositories returns a map of all resource repositories by server name
func (m *MCPManager) GetAllResourceRepositories() map[string]dive.DocumentRepository {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	repositories := make(map[string]dive.DocumentRepository)
	for serverName, server := range m.servers {
		if server.ResourceRepository != nil {
			repositories[serverName] = server.ResourceRepository
		}
	}
	return repositories
}
