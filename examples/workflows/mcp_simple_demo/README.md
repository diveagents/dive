# Simple MCP Demo Workflow

This workflow provides a basic introduction to MCP (Model Context Protocol) integration in Dive. It's designed to help you understand how MCP works and get started with MCP tools in your own workflows.

## What This Demo Shows

1. **MCP Configuration**: How to configure MCP servers in Dive YAML files
2. **Tool Discovery**: How agents automatically discover and access MCP tools
3. **Tool Usage**: Examples of calling MCP tools from workflow steps
4. **Error Handling**: What happens when MCP servers are unavailable
5. **Documentation Generation**: How to create guides and references

## Prerequisites

### Option 1: Run Without MCP Servers (Recommended for First Try)

You can run this demo even without any MCP servers running. The workflow will:
- Explain what MCP tools would be available if servers were running
- Show configuration examples
- Generate documentation about MCP usage
- Demonstrate the workflow structure

```bash
# Set up environment
export ANTHROPIC_API_KEY="your_anthropic_api_key"

# Run the demo
dive run simple_mcp_demo.yaml --workflow "MCP Tool Discovery"
```

### Option 2: Run With MCP Servers (Advanced)

To see actual MCP tool usage, you can set up MCP servers:

1. **HTTP MCP Server** (port 3000):
   ```bash
   # Example: Simple HTTP MCP server
   npm install -g @modelcontextprotocol/server-example
   mcp-server-example --port 3000
   ```

2. **Stdio MCP Server**:
   ```bash
   # Download and run a filesystem MCP server
   npm install -g @modelcontextprotocol/server-filesystem
   # Update the path in the YAML file to point to the server
   ```

## Usage Examples

### Basic Tool Discovery

```bash
# Discover what MCP tools are available
dive run simple_mcp_demo.yaml --workflow "MCP Tool Discovery" --input test_type=discovery
```

### Test Filesystem Tools

```bash
# Focus on filesystem MCP tools
dive run simple_mcp_demo.yaml --workflow "MCP Tool Discovery" --input test_type=filesystem
```

### Test Web Service Tools

```bash
# Focus on web service MCP tools  
dive run simple_mcp_demo.yaml --workflow "MCP Tool Discovery" --input test_type=web-service
```

### Configuration Demo

```bash
# Learn about MCP configuration patterns
dive run simple_mcp_demo.yaml --workflow "MCP Configuration Demo"
```

## What You'll Learn

### MCP Concepts

- **Model Context Protocol**: A standard for connecting AI assistants to external tools and data sources
- **Server Types**: HTTP servers (web services) vs Stdio servers (local processes)
- **Tool Discovery**: How agents automatically find and use available tools
- **Authentication**: How to securely pass credentials to MCP servers

### Dive Integration

- **Configuration**: How to define MCP servers in workflow YAML files
- **Tool Access**: How agents get access to MCP tools automatically
- **Error Handling**: What happens when MCP servers are unavailable
- **Filtering**: How to restrict which MCP tools are available to agents

### Practical Patterns

- **Multi-Server Setup**: Using multiple MCP servers in one workflow
- **Conditional Logic**: Running different steps based on available tools
- **Documentation**: Using workflows to generate usage guides
- **Testing**: Verifying MCP integration works correctly

## Expected Output

The workflows will generate:

1. **MCP Usage Guide** (`output/mcp_usage_guide.md`):
   - Explanation of discovered MCP tools
   - Step-by-step usage examples
   - Troubleshooting tips

2. **Configuration Reference** (`output/mcp_configuration_reference.md`):
   - Complete configuration options
   - Real-world examples
   - Security best practices

## Customizing the Demo

### Adding Your Own MCP Server

1. **Update the YAML configuration**:
   ```yaml
   MCPServers:
     - Type: http
       Name: my-server
       URL: "http://localhost:8080/mcp"
       AuthorizationToken: "${MY_API_TOKEN}"
   ```

2. **Set environment variables**:
   ```bash
   export MY_API_TOKEN="your_token_here"
   ```

3. **Run the demo**:
   ```bash
   dive run simple_mcp_demo.yaml
   ```

### Modifying Agent Behavior

Change the agent instructions to focus on specific aspects:

```yaml
Agents:
  - Name: MCP Explorer
    Instructions: |
      Focus on security aspects of MCP tools:
      1. Check what permissions each tool requires
      2. Identify potentially dangerous operations
      3. Suggest security best practices
      4. Document any security concerns
```

## Common Issues and Solutions

### No MCP Tools Found

This is normal if no MCP servers are running. The workflow will still:
- Generate documentation about MCP
- Show configuration examples  
- Explain how MCP would work

### Connection Errors

If you have MCP servers configured but getting connection errors:
1. Verify the server is running on the specified port
2. Check the URL and port number in the YAML
3. Ensure any required authentication tokens are set
4. Try running with `--log-level debug` for more details

### Tool Not Available

If specific tools aren't showing up:
1. Check the `AllowedTools` configuration
2. Verify the tool name matches what the server provides
3. Ensure `Enabled: true` is set in the ToolConfiguration

## Next Steps

After running this demo:

1. **Try the GitHub Assistant Example**: For a more advanced MCP workflow
2. **Build Your Own MCP Server**: Create custom tools for your specific needs
3. **Integrate Multiple Services**: Combine different MCP servers in one workflow
4. **Add Error Recovery**: Build robust workflows that handle MCP failures gracefully

## Resources

- [Model Context Protocol Specification](https://spec.modelcontextprotocol.io/)
- [Official MCP Servers](https://github.com/modelcontextprotocol/servers)
- [Dive Documentation](../../README.md)
- [MCP Community Examples](https://github.com/modelcontextprotocol/examples) 