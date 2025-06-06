# MCP OAuth Example

This example demonstrates comprehensive OAuth 2.0 support for MCP (Model Context Protocol) servers in Dive. It showcases various OAuth configurations, token management, and authentication flows.

## Features Demonstrated

### 🔐 OAuth 2.0 Authentication
- **Authorization Code Flow** with PKCE support
- **Public and Confidential Clients** configuration
- **Scope Management** for fine-grained permissions
- **State Parameter** validation for security
- **Interactive Browser Flow** with automatic callback handling

### 🗂️ Token Management
- **File-based Token Storage** for persistent authentication
- **In-memory Token Storage** for temporary sessions
- **Automatic Token Refresh** (when supported by servers)
- **Token Revocation** and cleanup

### 🔧 CLI Integration
- **Interactive OAuth Flow** via CLI commands
- **Token Status Monitoring** 
- **Server Connection Testing**
- **Bulk Token Management**

### 🌐 Multiple Server Support
- **GitHub** integration with repository and issue management
- **Linear** integration with issue tracking
- **Slack** integration with messaging capabilities
- **Custom Server** examples with hybrid authentication

## Configuration Examples

### 1. GitHub OAuth Configuration

```yaml
- Type: http
  Name: github-server
  URL: 'https://api.github.com/mcp'
  OAuth:
    ClientID: '${GITHUB_OAUTH_CLIENT_ID}'
    ClientSecret: '${GITHUB_OAUTH_CLIENT_SECRET}'
    RedirectURI: 'http://localhost:8085/oauth/callback'
    Scopes:
      - 'repo'
      - 'read:user'
    PKCEEnabled: true
    TokenStore:
      Type: file
      Path: '~/.dive/tokens/github-tokens.json'
```

### 2. Public Client (PKCE-only)

```yaml
- Type: http
  Name: linear-server
  URL: 'https://mcp.linear.app/sse'
  OAuth:
    ClientID: '${LINEAR_OAUTH_CLIENT_ID}'
    # No ClientSecret for public clients
    RedirectURI: 'http://localhost:8085/oauth/callback'
    Scopes: ['read', 'write']
    PKCEEnabled: true  # Essential for public clients
```

### 3. In-Memory Token Storage

```yaml
- Type: http
  Name: slack-server
  OAuth:
    # ... OAuth config ...
    TokenStore:
      Type: memory  # Tokens stored in memory only
```

## Setup Instructions

### 1. Environment Variables

Set up your OAuth credentials:

```bash
# GitHub OAuth App
export GITHUB_OAUTH_CLIENT_ID="your-github-client-id"
export GITHUB_OAUTH_CLIENT_SECRET="your-github-client-secret"

# Linear OAuth App  
export LINEAR_OAUTH_CLIENT_ID="your-linear-client-id"

# Slack OAuth App
export SLACK_OAUTH_CLIENT_ID="your-slack-client-id"
export SLACK_OAUTH_CLIENT_SECRET="your-slack-client-secret"
```

### 2. OAuth App Registration

For each service, register an OAuth application:

#### GitHub
1. Go to GitHub Settings → Developer settings → OAuth Apps
2. Create a new OAuth App
3. Set Authorization callback URL to: `http://localhost:8085/oauth/callback`
4. Note the Client ID and Client Secret

#### Linear
1. Go to Linear Settings → API → Create OAuth application
2. Set Redirect URI to: `http://localhost:8085/oauth/callback`
3. Note the Client ID (public client - no secret needed)

#### Slack
1. Go to Slack API → Your Apps → Create New App
2. Configure OAuth & Permissions
3. Add Redirect URL: `http://localhost:8085/oauth/callback`
4. Note Client ID and Client Secret

## Usage Examples

### 1. CLI Authentication

Start OAuth flow for a specific server:

```bash
# Authenticate with GitHub
dive mcp auth github-server

# Authenticate with Linear
dive mcp auth linear-server
```

### 2. Check OAuth Status

```bash
# Check all server statuses
dive mcp status

# Check specific server token status
dive mcp token status github-server
```

### 3. Token Management

```bash
# Refresh a token
dive mcp token refresh github-server

# Clear specific server tokens
dive mcp token clear github-server

# Clear all tokens
dive mcp token clear
```

### 4. Run OAuth-Enabled Workflow

```bash
# Run the OAuth demonstration workflow
dive run mcp_oauth_config.yaml oauth_demonstration
```

## OAuth Flow Details

### 1. Authorization Process

When you run `dive mcp auth [server-name]`:

1. **Configuration Loading**: Dive loads the OAuth configuration for the specified server
2. **Authorization URL Generation**: Creates authorization URL with state and PKCE parameters
3. **Browser Launch**: Automatically opens the authorization URL in your default browser
4. **User Consent**: You authorize the application in the browser
5. **Callback Handling**: Dive starts a local server to receive the authorization code
6. **Token Exchange**: Exchanges the authorization code for access/refresh tokens
7. **Token Storage**: Saves tokens according to your TokenStore configuration

### 2. Automatic Authentication

When using MCP servers in workflows:

1. **Token Check**: Dive checks for existing valid tokens
2. **Automatic Refresh**: Refreshes expired tokens if refresh token is available
3. **Interactive Flow**: Prompts for re-authentication if tokens are invalid
4. **Seamless Integration**: Continues with MCP server operations once authenticated

## Security Features

### 🔒 PKCE (Proof Key for Code Exchange)
- Essential for public clients (no client secret)
- Recommended for all OAuth flows
- Protects against authorization code interception

### 🔑 State Parameter Validation
- Prevents CSRF attacks
- Validates OAuth callback authenticity
- Required for all OAuth flows

### 💾 Secure Token Storage
- File-based storage with restricted permissions (0600)
- In-memory storage for temporary sessions
- Automatic cleanup of expired tokens

### 🌐 Localhost Callback Server
- Temporary local server for OAuth callbacks
- Automatic shutdown after token exchange
- User-friendly success page

## Troubleshooting

### Common Issues

1. **"OAuth configuration not found"**
   - Ensure your YAML configuration includes OAuth section
   - Verify environment variables are set correctly

2. **"Failed to open browser"**
   - The authorization URL will be printed to console
   - Manually copy and paste into your browser

3. **"Token not found"**
   - Run `dive mcp auth [server-name]` to authenticate
   - Check token file permissions and location

4. **"Invalid redirect URI"**
   - Ensure OAuth app is configured with correct callback URL
   - Default is `http://localhost:8085/oauth/callback`

### Debug Mode

Run with debug logging to see detailed OAuth interactions:

```bash
dive run mcp_oauth_config.yaml --log-level debug
```

## Advanced Configuration

### Custom Redirect URI

```yaml
OAuth:
  RedirectURI: 'http://localhost:9000/custom/callback'
```

### Additional OAuth Parameters

```yaml
OAuth:
  ExtraParams:
    prompt: 'consent'
    access_type: 'offline'
```

### Custom Token Store Path

```yaml
OAuth:
  TokenStore:
    Type: file
    Path: '/custom/path/tokens.json'
```

## Next Steps

This example provides a foundation for building OAuth-enabled MCP integrations:

1. **Production Deployment**: Configure production redirect URIs and secure token storage
2. **Multi-Tenant Support**: Implement user-specific token management
3. **Token Refresh Automation**: Build automated token refresh workflows
4. **Custom OAuth Providers**: Extend support for additional OAuth providers
5. **Enhanced Security**: Implement additional security measures like token rotation

## Security Considerations

- Never commit OAuth secrets to version control
- Use environment variables or secure secret management
- Implement proper token rotation and expiration
- Consider using short-lived tokens with refresh mechanisms
- Audit OAuth scope permissions regularly 