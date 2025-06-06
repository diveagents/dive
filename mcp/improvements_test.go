package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// MockServerConfig implements ServerConfig for testing
type MockServerConfig struct {
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

func (m *MockServerConfig) GetType() string               { return m.serverType }
func (m *MockServerConfig) GetName() string               { return m.name }
func (m *MockServerConfig) GetURL() string                { return m.url }
func (m *MockServerConfig) GetEnv() map[string]string     { return m.env }
func (m *MockServerConfig) GetArgs() []string             { return m.args }
func (m *MockServerConfig) GetAuthorizationToken() string { return m.authToken }
func (m *MockServerConfig) IsToolEnabled() bool           { return m.toolEnabled }
func (m *MockServerConfig) GetAllowedTools() []string     { return m.allowedTools }

// OAuth interface methods
func (m *MockServerConfig) IsOAuthEnabled() bool        { return m.oauthEnabled }
func (m *MockServerConfig) GetOAuthConfig() interface{} { return m.oauthConfig }

func TestMCPErrorTypes(t *testing.T) {
	// Test custom error types
	err := NewMCPError("test_operation", "test_server", ErrNotConnected)
	require.Contains(t, err.Error(), "test_operation")
	require.Contains(t, err.Error(), "test_server")
	require.True(t, IsNotConnectedError(err))

	// Test error unwrapping
	require.ErrorIs(t, err, ErrNotConnected)
}

func TestMCPClient_NewClient(t *testing.T) {
	config := &MockServerConfig{
		serverType:  "stdio",
		name:        "test-server",
		url:         "/bin/echo",
		toolEnabled: true,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, "test-server", client.config.GetName())
	require.False(t, client.IsConnected())
}

func TestMCPClient_ErrorHandling(t *testing.T) {
	config := &MockServerConfig{
		serverType:  "stdio",
		name:        "test-server",
		url:         "/bin/echo",
		toolEnabled: true,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	// Test operations on disconnected client
	ctx := context.Background()

	_, err = client.ListTools(ctx)
	require.Error(t, err)
	require.True(t, IsNotConnectedError(err))

	_, err = client.ListResources(ctx)
	require.Error(t, err)
	require.True(t, IsNotConnectedError(err))

	_, err = client.ReadResource(ctx, "test://resource")
	require.Error(t, err)
	require.True(t, IsNotConnectedError(err))

	_, err = client.CallTool(ctx, "test", map[string]interface{}{})
	require.Error(t, err)
	require.True(t, IsNotConnectedError(err))
}

func TestMCPResourceRepository(t *testing.T) {
	config := &MockServerConfig{
		serverType:  "stdio",
		name:        "test-server",
		url:         "/bin/echo",
		toolEnabled: true,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	repo := NewMCPResourceRepository(client, "test-server")
	require.NotNil(t, repo)

	ctx := context.Background()

	// Test operations on disconnected client
	_, err = repo.GetDocument(ctx, "test://resource")
	require.Error(t, err)
	require.True(t, IsNotConnectedError(err))

	_, err = repo.ListDocuments(ctx, nil)
	require.Error(t, err)
	require.True(t, IsNotConnectedError(err))

	exists, err := repo.Exists(ctx, "test://resource")
	require.Error(t, err)
	require.False(t, exists)
	require.True(t, IsNotConnectedError(err))

	// Test unsupported operations
	err = repo.PutDocument(ctx, nil)
	require.Error(t, err)
	require.True(t, IsUnsupportedOperationError(err))

	err = repo.DeleteDocument(ctx, nil)
	require.Error(t, err)
	require.True(t, IsUnsupportedOperationError(err))

	err = repo.RegisterDocument(ctx, "name", "path")
	require.Error(t, err)
	require.True(t, IsUnsupportedOperationError(err))
}

func TestMCPResourceDocumentConversion(t *testing.T) {
	config := &MockServerConfig{
		serverType:  "stdio",
		name:        "test-server",
		url:         "/bin/echo",
		toolEnabled: true,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	repo := NewMCPResourceRepository(client, "test-server")

	// Test text resource conversion
	textResource := &mcp.TextResourceContents{
		URI:      "file:///test.txt",
		MIMEType: "text/plain",
		Text:     "Hello, World!",
	}

	readResult := &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{textResource},
	}

	doc := repo.convertMCPResourceToDocument(readResult)
	require.Equal(t, "Hello, World!", doc.Content())
	require.Equal(t, "text/plain", doc.ContentType())
	require.Equal(t, "file:///test.txt", doc.Path())
	require.Equal(t, "test.txt", doc.Name())
	require.Contains(t, doc.ID(), "test-server")

	// Test blob resource conversion
	blobResource := &mcp.BlobResourceContents{
		URI:      "file:///test.bin",
		MIMEType: "application/octet-stream",
		Blob:     "binary data",
	}

	readResult = &mcp.ReadResourceResult{
		Contents: []mcp.ResourceContents{blobResource},
	}

	doc = repo.convertMCPResourceToDocument(readResult)
	require.Contains(t, doc.Content(), "Binary resource")
	require.Equal(t, "application/octet-stream", doc.ContentType())
	require.Equal(t, "file:///test.bin", doc.Path())
}

func TestMCPResourceMetadataConversion(t *testing.T) {
	config := &MockServerConfig{
		serverType:  "stdio",
		name:        "test-server",
		url:         "/bin/echo",
		toolEnabled: true,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	repo := NewMCPResourceRepository(client, "test-server")

	resource := mcp.Resource{
		URI:         "file:///test.txt",
		Name:        "Test Resource",
		Description: "A test resource",
		MIMEType:    "text/plain",
	}

	doc := repo.convertMCPResourceMetadataToDocument(resource)
	require.Equal(t, "", doc.Content()) // Content not loaded for listing
	require.Equal(t, "text/plain", doc.ContentType())
	require.Equal(t, "file:///test.txt", doc.Path())
	require.Equal(t, "test.txt", doc.Name())
	require.Contains(t, doc.ID(), "test-server")
}

func TestMCPManager_ResourceRepositories(t *testing.T) {
	manager := NewMCPManager()
	require.NotNil(t, manager)

	// Test getting repositories from empty manager
	repo := manager.GetResourceRepository("nonexistent")
	require.Nil(t, repo)

	repos := manager.GetAllResourceRepositories()
	require.Empty(t, repos)

	serverNames := manager.GetServerNames()
	require.Empty(t, serverNames)
}

func TestToolFilterConfiguration(t *testing.T) {
	// Test with tools disabled
	config := &MockServerConfig{
		serverType:  "stdio",
		name:        "test-server",
		url:         "/bin/echo",
		toolEnabled: false,
	}

	client, err := NewMCPClient(config)
	require.NoError(t, err)

	tools := []mcp.Tool{
		{Name: "tool1"},
		{Name: "tool2"},
	}

	filtered := client.filterTools(tools)
	require.Empty(t, filtered)

	// Test with allowed tools filter
	config.toolEnabled = true
	config.allowedTools = []string{"tool1"}

	filtered = client.filterTools(tools)
	require.Len(t, filtered, 1)
	require.Equal(t, "tool1", filtered[0].Name)

	// Test with no filter (all tools allowed)
	config.allowedTools = nil
	filtered = client.filterTools(tools)
	require.Len(t, filtered, 2)
}
