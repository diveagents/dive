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
	client    *client.Client
	config    ServerConfig
	tools     []mcp.Tool
	connected bool
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

	// Initialize the connection
	_, err = c.client.Initialize(ctx, mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{},
			ClientInfo: mcp.Implementation{
				Name:    "dive",
				Version: "1.0.0",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to initialize MCP client for server %s: %w", c.config.GetName(), err)
	}

	c.connected = true
	return nil
}

// ListTools retrieves available tools from the MCP server
func (c *MCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	if !c.connected {
		return nil, fmt.Errorf("MCP client not connected")
	}

	response, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools from server %s: %w", c.config.GetName(), err)
	}

	// Filter tools based on configuration
	tools := c.filterTools(response.Tools)
	c.tools = tools

	return tools, nil
}

// CallTool executes a tool on the MCP server
func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	if !c.connected {
		return nil, fmt.Errorf("MCP client not connected")
	}

	request := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}

	response, err := c.client.CallTool(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s on server %s: %w", name, c.config.GetName(), err)
	}

	return response, nil
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
