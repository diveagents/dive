# GitHub Assistant MCP Workflow

This example demonstrates how to use MCP (Model Context Protocol) tools within Dive workflows to interact with GitHub repositories. The workflow showcases two main use cases:

1. **Repository Analysis** - Comprehensive analysis of a GitHub repository
2. **Issue Creation** - Creating well-formatted GitHub issues

## Prerequisites

### 1. GitHub MCP Server

You'll need a GitHub MCP server running. You can use existing implementations like:

- [GitHub MCP Server](https://github.com/modelcontextprotocol/servers/tree/main/src/github) from the official MCP servers repository
- Or build your own using the [MCP specification](https://spec.modelcontextprotocol.io/)

### 2. Environment Setup

Set the following environment variables:

```bash
export GITHUB_TOKEN="your_github_personal_access_token"
export ANTHROPIC_API_KEY="your_anthropic_api_key"
```

### 3. MCP Server Configuration

The workflow assumes a GitHub MCP server is running at `http://localhost:8080/mcp`. Update the URL in the workflow file if your server runs elsewhere.

## Usage

### Repository Analysis

Analyze any GitHub repository with comprehensive code review and issue analysis:

```bash
# Basic repository analysis
dive run github_assistant.yaml --workflow "Repository Analysis" \
  --input repository_owner=diveagents \
  --input repository_name=dive

# Code review focused analysis
dive run github_assistant.yaml --workflow "Repository Analysis" \
  --input repository_owner=microsoft \
  --input repository_name=vscode \
  --input analysis_type=code-review

# Issue audit focused analysis
dive run github_assistant.yaml --workflow "Repository Analysis" \
  --input repository_owner=facebook \
  --input repository_name=react \
  --input analysis_type=issue-audit
```

### Create GitHub Issues

Create well-formatted GitHub issues with context research:

```bash
# Create a documentation issue
dive run github_assistant.yaml --workflow "Create Issue Workflow" \
  --input repository_owner=diveagents \
  --input repository_name=dive \
  --input issue_title="Add MCP integration examples" \
  --input issue_description="The project needs more examples showing how to integrate with MCP servers" \
  --input issue_type=documentation

# Create a feature request
dive run github_assistant.yaml --workflow "Create Issue Workflow" \
  --input repository_owner=diveagents \
  --input repository_name=dive \
  --input issue_title="Add workflow validation command" \
  --input issue_description="Need a command to validate workflow YAML files before execution" \
  --input issue_type=feature
```

## What This Demonstrates

### MCP Integration Features

1. **Server Configuration**: Shows how to configure MCP servers in Dive workflows
2. **Tool Access**: Demonstrates how agents can use MCP tools seamlessly
3. **Tool Filtering**: Shows how to restrict access to specific MCP tools
4. **Authentication**: Demonstrates passing authentication tokens to MCP servers

### Workflow Patterns

1. **Multi-Agent Collaboration**: Uses specialized agents (GitHub Analyst, Code Reviewer)
2. **Conditional Steps**: Steps that run based on input parameters
3. **Data Flow**: Passing data between workflow steps using variables
4. **Document Generation**: Creating output files with analysis results

### MCP Tool Usage

The workflow uses these GitHub MCP tools:

- `list_repositories` - List repositories for a user/organization
- `get_repository` - Get detailed repository information
- `list_issues` - List issues in a repository
- `create_issue` - Create new issues
- `get_file_contents` - Read file contents from repositories
- `search_code` - Search for code patterns in repositories

## Output

The workflows generate:

1. **Analysis Report**: `output/github_analysis_{owner}_{repo}.md`
2. **Issue Log**: `output/created_issue_{owner}_{repo}.md`

## Customization

### Adding More MCP Tools

To add more MCP tools, update the `AllowedTools` list in the configuration:

```yaml
AllowedTools: 
  - "list_repositories"
  - "get_repository"
  - "list_issues"
  - "create_issue"
  - "get_file_contents"
  - "search_code"
  - "create_pull_request"  # Add new tools here
  - "list_commits"
```

### Using Different MCP Servers

You can configure multiple MCP servers:

```yaml
MCPServers:
  - Type: http
    Name: github-server
    URL: "http://localhost:8080/mcp"
    AuthorizationToken: "${GITHUB_TOKEN}"
  - Type: stdio
    Name: filesystem-server
    URL: "python /path/to/filesystem_mcp_server.py"
```

### Custom Agent Instructions

Modify agent instructions to change behavior:

```yaml
Agents:
  - Name: Security Auditor
    Goal: Analyze repositories for security issues
    Instructions: |
      You are a security-focused code reviewer. Use GitHub MCP tools to:
      - Look for potential security vulnerabilities
      - Check for exposed secrets or credentials
      - Review dependency security
      - Assess access controls and permissions
```

## Troubleshooting

### Common Issues

1. **MCP Server Not Running**: Ensure your GitHub MCP server is running and accessible
2. **Authentication Errors**: Verify your `GITHUB_TOKEN` has the necessary permissions
3. **Tool Not Found**: Check that the tool name matches what your MCP server provides
4. **Connection Timeouts**: Increase timeout settings or check network connectivity

### Debug Mode

Run with debug logging to see detailed MCP interactions:

```bash
dive run github_assistant.yaml --log-level debug
```

This will show:
- MCP server connection details
- Tool discovery process
- Tool call requests and responses
- Error details if tools fail

## Next Steps

This example provides a foundation for building more complex MCP-powered workflows:

1. **Multi-Platform Integration**: Combine GitHub MCP with other platforms (Linear, Slack, etc.)
2. **Advanced Analytics**: Build more sophisticated code analysis using multiple MCP tools
3. **Automation Pipelines**: Create workflows that automatically respond to repository events
4. **Custom MCP Servers**: Build your own MCP servers for specialized functionality

For more information about MCP, visit the [Model Context Protocol documentation](https://modelcontextprotocol.io/). 