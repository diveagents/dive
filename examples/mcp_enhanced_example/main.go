package main

import (
	"context"
	"fmt"
	"log"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/mcp"
)

// ExampleServerConfig implements mcp.ServerConfig
type ExampleServerConfig struct {
	serverType   string
	name         string
	url          string
	env          map[string]string
	args         []string
	authToken    string
	toolEnabled  bool
	allowedTools []string
}

func (c *ExampleServerConfig) GetType() string               { return c.serverType }
func (c *ExampleServerConfig) GetName() string               { return c.name }
func (c *ExampleServerConfig) GetURL() string                { return c.url }
func (c *ExampleServerConfig) GetEnv() map[string]string     { return c.env }
func (c *ExampleServerConfig) GetArgs() []string             { return c.args }
func (c *ExampleServerConfig) GetAuthorizationToken() string { return c.authToken }
func (c *ExampleServerConfig) IsToolEnabled() bool           { return c.toolEnabled }
func (c *ExampleServerConfig) GetAllowedTools() []string     { return c.allowedTools }

func main() {
	ctx := context.Background()

	// Create server configurations
	configs := []mcp.ServerConfig{
		&ExampleServerConfig{
			serverType:  "stdio",
			name:        "filesystem-server",
			url:         "npx",
			args:        []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
			toolEnabled: true,
		},
		&ExampleServerConfig{
			serverType:   "stdio",
			name:         "git-server",
			url:          "npx",
			args:         []string{"-y", "@modelcontextprotocol/server-git", "--repository", "."},
			toolEnabled:  true,
			allowedTools: []string{"git_log", "git_diff"}, // Only allow specific tools
		},
	}

	// Create and initialize MCP manager
	manager := mcp.NewMCPManager()

	fmt.Println("🚀 Initializing MCP servers...")
	err := manager.InitializeServers(ctx, configs)
	if err != nil {
		log.Printf("Warning: Some servers failed to initialize: %v\n", err)
	}

	// Demonstrate enhanced capabilities
	demonstrateEnhancedCapabilities(ctx, manager)

	// Clean up
	fmt.Println("\n🧹 Cleaning up...")
	if err := manager.Close(); err != nil {
		log.Printf("Error during cleanup: %v\n", err)
	}
}

func demonstrateEnhancedCapabilities(ctx context.Context, manager *mcp.MCPManager) {
	// 1. Tool Discovery and Management
	fmt.Println("\n🔧 Available Tools:")
	allTools := manager.GetAllTools()
	for toolKey, tool := range allTools {
		annotations := tool.Annotations()
		fmt.Printf("  - %s: %s", toolKey, tool.Description())
		if annotations != nil && annotations.Title != "" {
			fmt.Printf(" (%s)", annotations.Title)
		}
		fmt.Println()
	}

	// 2. Resource Discovery and Access
	fmt.Println("\n📁 Available Resources:")
	resourceRepos := manager.GetAllResourceRepositories()
	for serverName, repo := range resourceRepos {
		fmt.Printf("  Server: %s\n", serverName)

		// List resources from this server
		listInput := &dive.ListDocumentInput{
			PathPrefix: "",
			Recursive:  false,
		}

		listOutput, err := repo.ListDocuments(ctx, listInput)
		if err != nil {
			if mcp.IsNotConnectedError(err) {
				fmt.Printf("    ❌ Server not connected\n")
			} else if mcp.IsUnsupportedOperationError(err) {
				fmt.Printf("    ℹ️  Resources not supported\n")
			} else {
				fmt.Printf("    ❌ Error listing resources: %v\n", err)
			}
			continue
		}

		if len(listOutput.Items) == 0 {
			fmt.Printf("    📂 No resources found\n")
			continue
		}

		for _, doc := range listOutput.Items {
			fmt.Printf("    📄 %s (%s)\n", doc.Name(), doc.ContentType())

			// Try to read the first few resources as examples
			if len(listOutput.Items) <= 3 {
				fullDoc, err := repo.GetDocument(ctx, doc.Path())
				if err != nil {
					fmt.Printf("      ❌ Error reading: %v\n", err)
				} else {
					content := fullDoc.Content()
					if len(content) > 100 {
						content = content[:100] + "..."
					}
					fmt.Printf("      📖 Content preview: %s\n", content)
				}
			}
		}
	}

	// 3. Error Handling Demonstration
	fmt.Println("\n⚠️  Error Handling Examples:")

	// Try to access a non-existent server
	nonExistentRepo := manager.GetResourceRepository("non-existent-server")
	if nonExistentRepo == nil {
		fmt.Println("  ✅ Gracefully handled non-existent server")
	}

	// Try unsupported operations
	if len(resourceRepos) > 0 {
		for serverName, repo := range resourceRepos {
			// Try an unsupported write operation
			err := repo.PutDocument(ctx, nil)
			if mcp.IsUnsupportedOperationError(err) {
				fmt.Printf("  ✅ Server %s correctly reports unsupported write operations\n", serverName)
			}
			break // Just test one server
		}
	}

	// 4. Server Status and Capabilities
	fmt.Println("\n📊 Server Status:")
	serverStatus := manager.GetServerStatus()
	for serverName, isConnected := range serverStatus {
		status := "❌ Disconnected"
		if isConnected {
			status = "✅ Connected"
		}
		fmt.Printf("  %s: %s\n", serverName, status)

		// Show server capabilities if connected
		if serverConn := getServerConnection(manager, serverName); serverConn != nil && isConnected {
			caps := serverConn.Client.GetServerCapabilities()
			if caps != nil {
				fmt.Printf("    Capabilities:")
				if caps.Tools != nil {
					fmt.Printf(" Tools")
				}
				if caps.Resources != nil {
					fmt.Printf(" Resources")
				}
				if caps.Prompts != nil {
					fmt.Printf(" Prompts")
				}
				fmt.Println()
			}
		}
	}

	// 5. Advanced Tool Usage with Error Handling
	fmt.Println("\n🛠️  Testing Tool Execution:")
	for toolKey, tool := range allTools {
		// Only test a few tools to avoid spam
		if len(allTools) > 3 && toolKey != "filesystem-server.list_directory" {
			continue
		}

		fmt.Printf("  Testing tool: %s\n", toolKey)

		// Try calling the tool with empty arguments (will likely fail, but demonstrates error handling)
		result, err := tool.Call(ctx, map[string]interface{}{})
		if err != nil {
			fmt.Printf("    ❌ Tool call failed (expected): %v\n", err)
		} else if result.IsError {
			fmt.Printf("    ⚠️  Tool returned error: %s\n", getFirstTextContent(result))
		} else {
			fmt.Printf("    ✅ Tool executed successfully: %s\n", getFirstTextContent(result))
		}
		break // Just test one tool
	}
}

// Helper function to get server connection (this would need to be added to the manager)
func getServerConnection(manager *mcp.MCPManager, serverName string) *mcp.MCPServerConnection {
	// This is a simplified version - in practice you'd add a method to MCPManager
	// to expose server connections safely
	return nil
}

// Helper function to extract first text content from tool result
func getFirstTextContent(result *dive.ToolResult) string {
	if len(result.Content) > 0 && result.Content[0].Type == dive.ToolResultContentTypeText {
		content := result.Content[0].Text
		if len(content) > 100 {
			return content[:100] + "..."
		}
		return content
	}
	return "No text content"
}
