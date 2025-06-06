package mcp

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

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
	// OAuth support
	IsOAuthEnabled() bool
	GetOAuthConfig() interface{} // Returns *config.MCPOAuthConfig but using interface{} to avoid circular import
}

// OAuthConfig represents OAuth configuration for internal use
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Scopes       []string
	PKCEEnabled  bool
	ExtraParams  map[string]string
}

// MCPClient wraps the mcp-go client library with OAuth support
type MCPClient struct {
	client             *client.Client
	config             ServerConfig
	oauthConfig        *OAuthConfig
	tools              []mcp.Tool
	resources          []mcp.Resource
	serverCapabilities *mcp.ServerCapabilities
	connected          bool
}

// NewMCPClient creates a new MCP client instance with OAuth support
func NewMCPClient(cfg ServerConfig) (*MCPClient, error) {
	mcpClient := &MCPClient{
		config:    cfg,
		connected: false,
	}

	// Set up OAuth if enabled
	if cfg.IsOAuthEnabled() {
		oauthCfg, err := mcpClient.convertOAuthConfig(cfg.GetOAuthConfig())
		if err != nil {
			return nil, fmt.Errorf("failed to configure OAuth: %w", err)
		}
		mcpClient.oauthConfig = oauthCfg
	}

	return mcpClient, nil
}

// convertOAuthConfig converts from config package OAuth config to internal OAuth config
func (c *MCPClient) convertOAuthConfig(configOAuth interface{}) (*OAuthConfig, error) {
	// Type assertion would be cleaner, but this avoids circular imports
	// In practice, you might use reflection or pass a conversion function

	// For now, we'll provide default OAuth configuration
	// This should be enhanced to properly extract values from the interface{}
	config := &OAuthConfig{
		ClientID:    "dive-client",
		RedirectURI: "http://localhost:8085/oauth/callback",
		Scopes:      []string{"mcp.read", "mcp.write"},
		PKCEEnabled: true,
	}

	// TODO: Implement proper conversion from configOAuth interface{}
	// This would involve either:
	// 1. Type assertion if we can import the config package
	// 2. Reflection to extract values
	// 3. A conversion interface/function pattern

	return config, nil
}

// Connect establishes connection to the MCP server with OAuth support
func (c *MCPClient) Connect(ctx context.Context) error {
	var err error

	switch c.config.GetType() {
	case "http":
		if c.config.IsOAuthEnabled() {
			err = c.connectWithOAuth(ctx)
		} else {
			c.client, err = client.NewStreamableHttpClient(c.config.GetURL())
		}
	case "stdio":
		// For stdio, URL contains the command to execute
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
		// Check if this is an OAuth authorization error
		if c.config.IsOAuthEnabled() && c.isOAuthAuthorizationError(err) {
			if authErr := c.handleOAuthAuthorization(ctx, err); authErr != nil {
				return fmt.Errorf("OAuth authorization failed for server %s: %w", c.config.GetName(), authErr)
			}
			// Retry connection after OAuth flow
			if err := c.client.Start(ctx); err != nil {
				return fmt.Errorf("failed to start MCP client after OAuth for server %s: %w", c.config.GetName(), err)
			}
		} else {
			return fmt.Errorf("failed to start MCP client for server %s: %w", c.config.GetName(), err)
		}
	}

	// Initialize the connection with enhanced capabilities
	if err := c.initializeConnection(ctx); err != nil {
		return err
	}

	c.connected = true
	return nil
}

// connectWithOAuth creates an OAuth-enabled HTTP client
func (c *MCPClient) connectWithOAuth(ctx context.Context) error {
	if c.oauthConfig == nil {
		return fmt.Errorf("OAuth configuration is nil")
	}

	// Create a memory token store for the OAuth client
	tokenStore := client.NewMemoryTokenStore()

	// Create OAuth configuration for mcp-go client
	oauthConfig := client.OAuthConfig{
		ClientID:     c.oauthConfig.ClientID,
		ClientSecret: c.oauthConfig.ClientSecret,
		RedirectURI:  c.oauthConfig.RedirectURI,
		Scopes:       c.oauthConfig.Scopes,
		TokenStore:   tokenStore,
		PKCEEnabled:  c.oauthConfig.PKCEEnabled,
	}

	var err error
	c.client, err = client.NewOAuthStreamableHttpClient(c.config.GetURL(), oauthConfig)
	return err
}

// isOAuthAuthorizationError checks if an error indicates OAuth authorization is required
func (c *MCPClient) isOAuthAuthorizationError(err error) bool {
	if c.client == nil {
		return false
	}
	return client.IsOAuthAuthorizationRequiredError(err)
}

// handleOAuthAuthorization handles the OAuth authorization flow
func (c *MCPClient) handleOAuthAuthorization(ctx context.Context, err error) error {
	log.Println("OAuth authorization required. Starting authorization flow...")

	// Get the OAuth handler from the error
	oauthHandler := client.GetOAuthHandler(err)
	if oauthHandler == nil {
		return fmt.Errorf("failed to get OAuth handler from error")
	}

	// Start a local server to handle the OAuth callback
	callbackChan := make(chan map[string]string, 1)
	server := c.startCallbackServer(callbackChan)
	defer func() {
		if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
			log.Printf("Error shutting down callback server: %v", shutdownErr)
		}
	}()

	// Generate PKCE code verifier and challenge if enabled
	var codeVerifier, codeChallenge string
	var state string
	var genErr error

	if c.oauthConfig.PKCEEnabled {
		codeVerifier, genErr = client.GenerateCodeVerifier()
		if genErr != nil {
			return fmt.Errorf("failed to generate code verifier: %w", genErr)
		}
		codeChallenge = client.GenerateCodeChallenge(codeVerifier)
	}

	// Generate state parameter
	state, genErr = client.GenerateState()
	if genErr != nil {
		return fmt.Errorf("failed to generate state: %w", genErr)
	}

	// Register client if needed
	if err := oauthHandler.RegisterClient(ctx, fmt.Sprintf("dive-%s", c.config.GetName())); err != nil {
		return fmt.Errorf("failed to register OAuth client: %w", err)
	}

	// Get the authorization URL
	authURL, err := oauthHandler.GetAuthorizationURL(ctx, state, codeChallenge)
	if err != nil {
		return fmt.Errorf("failed to get authorization URL: %w", err)
	}

	// Open the browser to the authorization URL
	log.Printf("Opening browser to: %s", authURL)
	if err := c.openBrowser(authURL); err != nil {
		log.Printf("Failed to open browser automatically: %v", err)
		log.Printf("Please open the following URL in your browser: %s", authURL)
	}

	// Wait for the callback
	log.Println("Waiting for authorization callback...")
	params := <-callbackChan

	// Verify state parameter
	if params["state"] != state {
		return fmt.Errorf("state mismatch: expected %s, got %s", state, params["state"])
	}

	// Exchange the authorization code for a token
	code := params["code"]
	if code == "" {
		return fmt.Errorf("no authorization code received")
	}

	log.Println("Exchanging authorization code for token...")
	if err := oauthHandler.ProcessAuthorizationResponse(ctx, code, state, codeVerifier); err != nil {
		return fmt.Errorf("failed to process authorization response: %w", err)
	}

	log.Println("OAuth authorization successful!")
	return nil
}

// startCallbackServer starts a local HTTP server to handle the OAuth callback
func (c *MCPClient) startCallbackServer(callbackChan chan<- map[string]string) *http.Server {
	server := &http.Server{
		Addr: ":0", // Use random available port
	}

	http.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		// Extract query parameters
		params := make(map[string]string)
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				params[key] = values[0]
			}
		}

		// Send parameters to the channel
		select {
		case callbackChan <- params:
		default:
			// Channel is full, ignore
		}

		// Respond to the user
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte(`
			<html>
				<body>
					<h1>Authorization Successful</h1>
					<p>You can now close this window and return to the application.</p>
					<script>window.close();</script>
				</body>
			</html>
		`))
		if err != nil {
			log.Printf("Error writing response: %v", err)
		}
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return server
}

// openBrowser opens the default browser to the specified URL
func (c *MCPClient) openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}

// initializeConnection initializes the MCP connection
func (c *MCPClient) initializeConnection(ctx context.Context) error {
	// Add a timeout to prevent hanging
	initCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	initResponse, err := c.client.Initialize(initCtx, mcp.InitializeRequest{
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
		fmt.Printf("ERROR INITIALIZING CONNECTION %s: %v\n", c.config.GetName(), err)
		if initCtx.Err() == context.DeadlineExceeded {
			return NewMCPError("initialize", c.config.GetName(), fmt.Errorf("initialization timeout after 30s: %w", ErrInitializationFailed))
		}
		return NewMCPError("initialize", c.config.GetName(), fmt.Errorf("%w: %v", ErrInitializationFailed, err))
	}

	// Store server capabilities for later use
	c.serverCapabilities = &initResponse.Capabilities
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
