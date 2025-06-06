package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// MockOAuthServerConfig implements ServerConfig for testing OAuth functionality
type MockOAuthServerConfig struct {
	serverType   string
	name         string
	url          string
	env          map[string]string
	args         []string
	authToken    string
	toolEnabled  bool
	allowedTools []string
	oauthEnabled bool
	oauthConfig  interface{}
}

func (m *MockOAuthServerConfig) GetType() string               { return m.serverType }
func (m *MockOAuthServerConfig) GetName() string               { return m.name }
func (m *MockOAuthServerConfig) GetURL() string                { return m.url }
func (m *MockOAuthServerConfig) GetEnv() map[string]string     { return m.env }
func (m *MockOAuthServerConfig) GetArgs() []string             { return m.args }
func (m *MockOAuthServerConfig) GetAuthorizationToken() string { return m.authToken }
func (m *MockOAuthServerConfig) IsToolEnabled() bool           { return m.toolEnabled }
func (m *MockOAuthServerConfig) GetAllowedTools() []string     { return m.allowedTools }
func (m *MockOAuthServerConfig) IsOAuthEnabled() bool          { return m.oauthEnabled }
func (m *MockOAuthServerConfig) GetOAuthConfig() interface{}   { return m.oauthConfig }

func TestNewMCPClient_WithoutOAuth(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType:   "http",
		name:         "test-server",
		url:          "https://example.com",
		oauthEnabled: false,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Nil(t, client.oauthConfig)
	require.Equal(t, config, client.config)
	require.False(t, client.connected)
}

func TestNewMCPClient_WithOAuth(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType:   "http",
		name:         "oauth-server",
		url:          "https://example.com",
		oauthEnabled: true,
		oauthConfig:  map[string]interface{}{"client_id": "test-client"},
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.NotNil(t, client.oauthConfig)
	require.Equal(t, "dive-client", client.oauthConfig.ClientID)
	require.Equal(t, "http://localhost:8085/oauth/callback", client.oauthConfig.RedirectURI)
	require.True(t, client.oauthConfig.PKCEEnabled)
	require.Equal(t, []string{"mcp.read", "mcp.write"}, client.oauthConfig.Scopes)
}

func TestConvertOAuthConfig(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType: "http",
		name:       "test-server",
		url:        "https://example.com",
	}

	client := &MCPClient{config: config}

	// Test with basic OAuth config
	oauthConfig, err := client.convertOAuthConfig(map[string]interface{}{
		"client_id": "test-client-id",
	})

	require.NoError(t, err)
	require.NotNil(t, oauthConfig)
	require.Equal(t, "dive-client", oauthConfig.ClientID)
	require.Equal(t, "http://localhost:8085/oauth/callback", oauthConfig.RedirectURI)
	require.True(t, oauthConfig.PKCEEnabled)
	require.Contains(t, oauthConfig.Scopes, "mcp.read")
	require.Contains(t, oauthConfig.Scopes, "mcp.write")
}

func TestMCPClient_OAuth_IsConnected(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType: "http",
		name:       "test-server",
		url:        "https://example.com",
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)
	require.False(t, client.IsConnected())

	// Simulate connection
	client.connected = true
	require.True(t, client.IsConnected())
}

func TestMCPClient_OAuth_Close(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType: "http",
		name:       "test-server",
		url:        "https://example.com",
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	client.connected = true
	err = client.Close()
	require.NoError(t, err)
}

func TestMCPClient_OAuth_ListTools_NotConnected(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType: "http",
		name:       "test-server",
		url:        "https://example.com",
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	tools, err := client.ListTools(ctx)
	require.Error(t, err)
	require.Nil(t, tools)
	require.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_OAuth_ListResources_NotConnected(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType: "http",
		name:       "test-server",
		url:        "https://example.com",
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	resources, err := client.ListResources(ctx)
	require.Error(t, err)
	require.Nil(t, resources)
	require.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_OAuth_CallTool_NotConnected(t *testing.T) {
	config := &MockOAuthServerConfig{
		serverType: "http",
		name:       "test-server",
		url:        "https://example.com",
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	ctx := context.Background()
	result, err := client.CallTool(ctx, "test-tool", map[string]interface{}{})
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "not connected")
}

func TestMCPClient_OAuthConfiguration_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *MockOAuthServerConfig
		expectOAuth bool
		expectError bool
	}{
		{
			name: "no OAuth configuration",
			config: &MockOAuthServerConfig{
				serverType:   "http",
				name:         "test-server",
				url:          "https://example.com",
				oauthEnabled: false,
			},
			expectOAuth: false,
			expectError: false,
		},
		{
			name: "OAuth enabled",
			config: &MockOAuthServerConfig{
				serverType:   "http",
				name:         "oauth-server",
				url:          "https://example.com",
				oauthEnabled: true,
				oauthConfig:  map[string]interface{}{"client_id": "test-client"},
			},
			expectOAuth: true,
			expectError: false,
		},
		{
			name: "stdio with OAuth (should work)",
			config: &MockOAuthServerConfig{
				serverType:   "stdio",
				name:         "stdio-oauth-server",
				url:          "test-command",
				oauthEnabled: true,
				oauthConfig:  map[string]interface{}{"client_id": "test-client"},
			},
			expectOAuth: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewMCPClient(tt.config)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)

			if tt.expectOAuth {
				require.NotNil(t, client.oauthConfig)
			} else {
				require.Nil(t, client.oauthConfig)
			}
		})
	}
}
