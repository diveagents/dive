# GitHub Search Tool for Dive

This package provides a GitHub search tool for the Dive agent framework. It allows semantic search of GitHub repositories, enabling agents to find relevant code, repository information, pull requests, and issues.

## Features

- Semantic search of GitHub repositories
- Support for different content types:
  - Code files
  - Repository information
  - Pull requests
  - Issues
- Fixed repository mode (search within a specific repository)
- Dynamic repository mode (search any repository)

## Usage

### Basic Usage

```go
import (
    "github.com/getstingrai/dive"
    "github.com/getstingrai/dive/llm"
    "github.com/getstingrai/dive/providers/anthropic"
    "github.com/getstingrai/dive/tools"
)

// Create a GitHub search tool
githubSearchTool := tools.NewGitHubSearch("your-github-token", 10)

// Create an agent with the GitHub search tool
agent := dive.NewAgent(dive.AgentOptions{
    Name: "GitHub Search Agent",
    Role: dive.Role{
        Description:  "An agent that can search GitHub repositories",
        AcceptsChats: true,
    },
    LLM:   llmProvider,
    Tools: []llm.Tool{githubSearchTool},
})
```

### Fixed Repository Mode

If you want to limit searches to a specific repository:

```go
// Create a GitHub search tool for a specific repository
githubSearchTool := tools.NewFixedRepoGitHubSearch(
    "your-github-token",
    "owner/repo",
    10,                              // Max results
    []string{"code", "repo"}         // Content types to include
)
```

### Example Queries

When using the tool through an agent, you can ask questions like:

- "Search for 'agent framework' in the 'getstingrai/dive' repository"
- "Find code related to 'embeddings' in the 'openai/openai-cookbook' repository"
- "Look for pull requests about 'bug fixes' in the 'tensorflow/tensorflow' repository"

## Implementation Details

### Components

1. **GitHub Client**: Handles API requests to GitHub
2. **Embedding Provider**: Generates vector embeddings for content
3. **Vector Store**: Stores and searches embeddings
4. **GitHub Search Tool**: Integrates with Dive's tool system

### Limitations

- The current implementation uses a simple embedding model that should be replaced with a more sophisticated one in production
- The GitHub client does not handle pagination or rate limiting robustly
- Repository content is fetched and indexed on-demand, which may cause delays for large repositories

## Future Improvements

- Add support for more sophisticated embedding models
- Implement better caching of repository content
- Add support for recursive directory traversal
- Improve error handling and rate limit management
- Add support for authentication with GitHub Apps or OAuth 