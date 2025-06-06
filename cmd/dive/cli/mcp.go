package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/diveagents/dive/config"
	"github.com/diveagents/dive/mcp"
)

// mcpCmd represents the mcp command for managing MCP server connections
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP (Model Context Protocol) server connections",
	Long: `Commands for managing MCP server connections, including OAuth authentication,
token management, and server status checking.`,
}

// mcpAuthCmd handles OAuth authentication flow for MCP servers
var mcpAuthCmd = &cobra.Command{
	Use:   "auth [server-name]",
	Short: "Authenticate with an MCP server using OAuth",
	Long: `Start the OAuth authentication flow for the specified MCP server.
This will open a browser window for user authorization and store the resulting tokens.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := args[0]

		// Load configuration to find the server
		env, err := config.ParseFile("")
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		// Find the server configuration
		var serverConfig config.MCPServer
		var found bool

		for _, provider := range env.Config.Providers {
			for _, server := range provider.MCPServers {
				if server.Name == serverName {
					serverConfig = server
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			return fmt.Errorf("MCP server '%s' not found in configuration", serverName)
		}

		if !serverConfig.IsOAuthEnabled() {
			return fmt.Errorf("OAuth is not configured for server '%s'", serverName)
		}

		// Create MCP client and trigger OAuth flow
		client, err := mcp.NewMCPClient(serverConfig)
		if err != nil {
			return fmt.Errorf("failed to create MCP client: %w", err)
		}

		fmt.Printf("Starting OAuth authentication for server '%s'...\n", serverName)

		ctx := context.Background()
		if err := client.Connect(ctx); err != nil {
			return fmt.Errorf("OAuth authentication failed: %w", err)
		}

		fmt.Printf("✓ OAuth authentication successful for server '%s'\n", serverName)

		// Clean up
		client.Close()
		return nil
	},
}

// mcpTokenCmd manages OAuth tokens for MCP servers
var mcpTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage OAuth tokens for MCP servers",
	Long:  `Commands for managing OAuth tokens including refresh, revoke, and status checking.`,
}

// mcpTokenRefreshCmd refreshes OAuth tokens for MCP servers
var mcpTokenRefreshCmd = &cobra.Command{
	Use:   "refresh [server-name]",
	Short: "Refresh OAuth token for an MCP server",
	Long:  `Refresh the OAuth token for the specified MCP server.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := args[0]
		fmt.Printf("Token refresh for server '%s' is not yet implemented\n", serverName)
		// TODO: Implement token refresh logic
		return nil
	},
}

// mcpTokenStatusCmd shows OAuth token status for MCP servers
var mcpTokenStatusCmd = &cobra.Command{
	Use:   "status [server-name]",
	Short: "Show OAuth token status for MCP servers",
	Long:  `Display the OAuth token status for the specified server or all servers.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			serverName := args[0]
			fmt.Printf("Token status for server '%s' is not yet implemented\n", serverName)
		} else {
			fmt.Println("Token status for all servers is not yet implemented")
		}
		// TODO: Implement token status logic
		return nil
	},
}

// mcpTokenClearCmd clears stored OAuth tokens
var mcpTokenClearCmd = &cobra.Command{
	Use:   "clear [server-name]",
	Short: "Clear stored OAuth tokens",
	Long:  `Clear stored OAuth tokens for the specified server or all servers.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			serverName := args[0]
			return clearTokensForServer(serverName)
		} else {
			return clearAllTokens()
		}
	},
}

// mcpStatusCmd shows status of MCP server connections
var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of MCP server connections",
	Long:  `Display the connection status and capabilities of configured MCP servers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		env, err := config.ParseFile("")
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		fmt.Println("MCP Server Status:")
		fmt.Println("==================")

		// Check each configured server
		for _, provider := range env.Config.Providers {
			if len(provider.MCPServers) == 0 {
				continue
			}

			fmt.Printf("\nProvider: %s\n", provider.Name)
			for _, server := range provider.MCPServers {
				fmt.Printf("  %s (%s): ", server.Name, server.Type)

				if server.IsOAuthEnabled() {
					fmt.Print("OAuth configured")
				} else if server.AuthorizationToken != "" {
					fmt.Print("Token auth configured")
				} else {
					fmt.Print("No auth configured")
				}

				// TODO: Add actual connection testing
				fmt.Println(" - Connection status: Not tested")
			}
		}

		return nil
	},
}

// clearTokensForServer clears tokens for a specific server
func clearTokensForServer(serverName string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	tokenPath := filepath.Join(homeDir, ".dive", "tokens", "mcp-tokens.json")

	// TODO: Implement server-specific token clearing
	fmt.Printf("Clearing tokens for server '%s' from %s (not yet implemented)\n", serverName, tokenPath)
	return nil
}

// clearAllTokens clears all stored tokens
func clearAllTokens() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	tokenPath := filepath.Join(homeDir, ".dive", "tokens", "mcp-tokens.json")

	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		fmt.Println("No token file found - nothing to clear")
		return nil
	}

	if err := os.Remove(tokenPath); err != nil {
		return fmt.Errorf("failed to remove token file: %w", err)
	}

	fmt.Printf("✓ Cleared all OAuth tokens from %s\n", tokenPath)
	return nil
}

func init() {
	// Add mcp command to root
	rootCmd.AddCommand(mcpCmd)

	// Add subcommands to mcp
	mcpCmd.AddCommand(mcpAuthCmd)
	mcpCmd.AddCommand(mcpTokenCmd)
	mcpCmd.AddCommand(mcpStatusCmd)

	// Add token subcommands
	mcpTokenCmd.AddCommand(mcpTokenRefreshCmd)
	mcpTokenCmd.AddCommand(mcpTokenStatusCmd)
	mcpTokenCmd.AddCommand(mcpTokenClearCmd)
}
