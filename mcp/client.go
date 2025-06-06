package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// ServerConfig represents the configuration for an MCP server
// This interface avoids importing the config package to prevent circular dependencies
type ServerConfig interface {
	GetType() string
	GetName() string
	GetURL() string
	GetEnv() map[string]string
	GetArgs() []string
	GetAuthorizationToken() string
	IsToolEnabled() bool
	GetAllowedTools() []string
}

// MCPClient wraps the mcp-go client library
type MCPClient struct {
	client             *client.Client
	config             ServerConfig
	tools              []mcp.Tool
	resources          []mcp.Resource
	serverCapabilities *mcp.ServerCapabilities
	connected          bool
}

// NewMCPClient creates a new MCP client instance
func NewMCPClient(cfg ServerConfig) (*MCPClient, error) {
	return &MCPClient{
		config:    cfg,
		connected: false,
	}, nil
}

// Connect establishes connection to the MCP server
func (c *MCPClient) Connect(ctx context.Context) error {
	var err error

	switch c.config.GetType() {
	case "http":
		c.client, err = client.NewStreamableHttpClient(c.config.GetURL())
	case "stdio":
		// For stdio, URL contains the command to execute
		// Get environment variables and arguments from config
		envMap := c.config.GetEnv()
		args := c.config.GetArgs()

		// Convert environment map to slice of "KEY=VALUE" strings
		env := make([]string, 0, len(envMap))
		for key, value := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}

		c.client, err = client.NewStdioMCPClient(c.config.GetURL(), env, args...)
	default:
		return fmt.Errorf("unsupported MCP server type: %s", c.config.GetType())
	}

	if err != nil {
		return fmt.Errorf("failed to create MCP client for server %s: %w", c.config.GetName(), err)
	}

	// Start the connection
	if err := c.client.Start(ctx); err != nil {
		return fmt.Errorf("failed to start MCP client for server %s: %w", c.config.GetName(), err)
	}

	// Initialize the connection with enhanced capabilities
	initResponse, err := c.client.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities: mcp.ClientCapabilities{
				Experimental: make(map[string]any),
				Roots: &struct {
					ListChanged bool `json:"listChanged,omitempty"`
				}{
					ListChanged: true,
				},
				Sampling: &struct{}{},
			},
			ClientInfo: mcp.Implementation{
				Name:    "dive",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		return NewMCPError("initialize", c.config.GetName(), fmt.Errorf("%w: %v", ErrInitializationFailed, err))
	}

	// Store server capabilities for later use
	c.serverCapabilities = &initResponse.Capabilities
	c.connected = true
	return nil
}

// ListTools retrieves available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	if !c.connected {
		return nil, NewMCPError("list_tools", c.config.GetName(), ErrNotConnected)
	}

	response, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, NewMCPError("list_tools", c.config.GetName(), err)
	}

	// Filter tools based on configuration
	tools := c.filterTools(response.Tools)
	c.tools = tools

	return tools, nil
}

// CallTool executes a tool on the MCP server
func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if !c.connected {
		return nil, NewMCPError("call_tool", c.config.GetName(), ErrNotConnected)
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}

	response, err := c.client.CallTool(ctx, request)
	if err != nil {
		return nil, NewMCPError("call_tool", c.config.GetName(), err)
	}

	return response, nil
}

// ListResources retrieves available resources from the MCP server
func (c *MCPClient) ListResources(ctx context.Context) ([]mcp.Resource, error) {
	if !c.connected {
		return nil, NewMCPError("list_resources", c.config.GetName(), ErrNotConnected)
	}

	// Check if server supports resources
	if c.serverCapabilities == nil || c.serverCapabilities.Resources == nil {
		return nil, NewMCPError("list_resources", c.config.GetName(), ErrUnsupportedOperation)
	}

	response, err := c.client.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return nil, NewMCPError("list_resources", c.config.GetName(), err)
	}

	c.resources = response.Resources
	return response.Resources, nil
}

// ReadResource reads a specific resource from the MCP server
func (c *MCPClient) ReadResource(ctx context.Context, uri string) (*mcp.ReadResourceResult, error) {
	if !c.connected {
		return nil, NewMCPError("read_resource", c.config.GetName(), ErrNotConnected)
	}

	// Check if server supports resources
	if c.serverCapabilities == nil || c.serverCapabilities.Resources == nil {
		return nil, NewMCPError("read_resource", c.config.GetName(), ErrUnsupportedOperation)
	}

	request := mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: uri,
		},
	}

	response, err := c.client.ReadResource(ctx, request)
	if err != nil {
		return nil, NewMCPError("read_resource", c.config.GetName(), err)
	}

	return response, nil
}

// GetResources returns the cached list of resources
func (c *MCPClient) GetResources() []mcp.Resource {
	return c.resources
}

// GetServerCapabilities returns the server capabilities
func (c *MCPClient) GetServerCapabilities() *mcp.ServerCapabilities {
	return c.serverCapabilities
}

// GetTools returns the cached list of tools
func (c *MCPClient) GetTools() []mcp.Tool {
	return c.tools
}

// IsConnected returns whether the client is connected
func (c *MCPClient) IsConnected() bool {
	return c.connected
}

// Close closes the MCP client connection
func (c *MCPClient) Close() error {
	if c.client != nil && c.connected {
		c.connected = false
		// Note: The mcp-go client doesn't seem to have a Close method
		// This might need to be updated based on the actual API
	}
	return nil
}

// filterTools filters tools based on the server's ToolConfiguration
func (c *MCPClient) filterTools(tools []mcp.Tool) []mcp.Tool {
	// If tools are explicitly disabled, return no tools
	if !c.config.IsToolEnabled() {
		return []mcp.Tool{}
	}

	// If no allowed tools specified, return all tools
	allowedTools := c.config.GetAllowedTools()
	if len(allowedTools) == 0 {
		return tools
	}

	// Filter tools based on AllowedTools list
	allowedMap := make(map[string]bool)
	for _, toolName := range allowedTools {
		allowedMap[toolName] = true
	}

	var filteredTools []mcp.Tool
	for _, tool := range tools {
		if allowedMap[tool.Name] {
			filteredTools = append(filteredTools, tool)
		}
	}

	return filteredTools
}
