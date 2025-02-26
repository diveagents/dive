package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/tools/github"
)

var _ llm.Tool = &GitHubSearch{}

// GitHubSearchInput represents the input parameters for the GitHub search tool
type GitHubSearchInput struct {
	SearchQuery  string   `json:"search_query"`
	GitHubRepo   string   `json:"github_repo,omitempty"`
	ContentTypes []string `json:"content_types,omitempty"`
}

// GitHubSearch implements a tool for semantic search of GitHub repositories
type GitHubSearch struct {
	defaultRepo  string
	contentTypes []string
	ghClient     *github.Client
	embeddings   *github.SimpleEmbeddingProvider
	vectorStore  *github.InMemoryVectorStore
	maxResults   int
	hasFixedRepo bool
}

// NewGitHubSearch creates a new GitHub search tool
func NewGitHubSearch(ghToken string, maxResults int) *GitHubSearch {
	if maxResults <= 0 {
		maxResults = 10
	}

	return &GitHubSearch{
		ghClient:     github.New(ghToken),
		embeddings:   github.NewSimpleEmbeddingProvider(),
		vectorStore:  github.NewInMemoryVectorStore(),
		maxResults:   maxResults,
		contentTypes: []string{"code", "repo", "pr", "issue"},
	}
}

// NewFixedRepoGitHubSearch creates a GitHub search tool with a fixed repository
func NewFixedRepoGitHubSearch(ghToken string, repo string, maxResults int, contentTypes []string) *GitHubSearch {
	if maxResults <= 0 {
		maxResults = 10
	}

	if contentTypes == nil {
		contentTypes = []string{"code", "repo", "pr", "issue"}
	}

	tool := &GitHubSearch{
		defaultRepo:  repo,
		contentTypes: contentTypes,
		ghClient:     github.New(ghToken),
		embeddings:   github.NewSimpleEmbeddingProvider(),
		vectorStore:  github.NewInMemoryVectorStore(),
		maxResults:   maxResults,
		hasFixedRepo: true,
	}

	// Initialize the repository content in the vector store
	// This would typically be done asynchronously or at startup
	go tool.AddRepository(context.Background(), repo, contentTypes)

	return tool
}

// Definition returns the tool definition
func (t *GitHubSearch) Definition() *llm.ToolDefinition {
	var def *llm.ToolDefinition

	if t.hasFixedRepo {
		// Fixed repository version
		def = &llm.ToolDefinition{
			Name:        "github_search",
			Description: fmt.Sprintf("A tool that can be used to semantic search a query in the %s GitHub repository's content. This is not the GitHub API, but instead a tool that provides semantic search capabilities.", t.defaultRepo),
			Parameters: llm.Schema{
				Type:     "object",
				Required: []string{"search_query"},
				Properties: map[string]*llm.SchemaProperty{
					"search_query": {
						Type:        "string",
						Description: "Mandatory search query you want to use to search the github repo's content",
					},
				},
			},
		}
	} else {
		// Dynamic repository version
		def = &llm.ToolDefinition{
			Name:        "github_search",
			Description: "A tool that can be used to semantic search a query from a GitHub repository's content. This is not the GitHub API, but instead a tool that provides semantic search capabilities.",
			Parameters: llm.Schema{
				Type:     "object",
				Required: []string{"search_query", "github_repo", "content_types"},
				Properties: map[string]*llm.SchemaProperty{
					"search_query": {
						Type:        "string",
						Description: "Mandatory search query you want to use to search the github repo's content",
					},
					"github_repo": {
						Type:        "string",
						Description: "Mandatory github repository you want to search (e.g., 'owner/repo')",
					},
					"content_types": {
						Type:        "array",
						Description: "Mandatory content types you want to be included in search, options: [code, repo, pr, issue]",
						Items: &llm.SchemaProperty{
							Type: "string",
						},
					},
				},
			},
		}
	}

	return def
}

// Call executes the GitHub search
func (t *GitHubSearch) Call(ctx context.Context, input string) (string, error) {
	var params GitHubSearchInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Use default repo if fixed or not provided
	repo := t.defaultRepo
	if !t.hasFixedRepo {
		if params.GitHubRepo == "" {
			return "", fmt.Errorf("github_repo is required")
		}
		repo = params.GitHubRepo

		// If this is a dynamic repo search, we need to index the repo first
		contentTypes := t.contentTypes
		if len(params.ContentTypes) > 0 {
			contentTypes = params.ContentTypes
		}

		// Convert string content types to ContentType
		var ghContentTypes []github.ContentType
		for _, ct := range contentTypes {
			switch ct {
			case "code":
				ghContentTypes = append(ghContentTypes, github.ContentTypeCode)
			case "repo":
				ghContentTypes = append(ghContentTypes, github.ContentTypeRepo)
			case "pr":
				ghContentTypes = append(ghContentTypes, github.ContentTypePR)
			case "issue":
				ghContentTypes = append(ghContentTypes, github.ContentTypeIssue)
			}
		}

		// Fetch and index the repository content
		if err := t.indexRepository(ctx, repo, ghContentTypes); err != nil {
			return "", fmt.Errorf("failed to index repository: %w", err)
		}
	}

	// Generate embedding for the search query
	embedding, err := t.embeddings.GenerateEmbedding(ctx, params.SearchQuery)
	if err != nil {
		return "", fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search the vector store
	results, err := t.vectorStore.Search(ctx, embedding, t.maxResults)
	if err != nil {
		return "", fmt.Errorf("failed to search: %w", err)
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for query '%s' in repository '%s'", params.SearchQuery, repo), nil
	}

	// Format the results
	var lines []string
	lines = append(lines, fmt.Sprintf("# GitHub Search Results for '%s' in '%s'", params.SearchQuery, repo))

	// Get content types for display
	contentTypes := t.contentTypes
	if !t.hasFixedRepo && len(params.ContentTypes) > 0 {
		contentTypes = params.ContentTypes
	}
	lines = append(lines, fmt.Sprintf("Content types: %s", strings.Join(contentTypes, ", ")))
	lines = append(lines, "")

	for i, result := range results {
		lines = append(lines, fmt.Sprintf("## Result %d", i+1))

		// Extract metadata if available
		filePath := result.Path
		contentType := result.Type

		if filePath != "" {
			lines = append(lines, fmt.Sprintf("**Path**: %s", filePath))
		}

		if contentType != "" {
			lines = append(lines, fmt.Sprintf("**Type**: %s", contentType))
		}

		if result.URL != "" {
			lines = append(lines, fmt.Sprintf("**URL**: %s", result.URL))
		}

		lines = append(lines, "")
		lines = append(lines, "```")
		lines = append(lines, result.Content)
		lines = append(lines, "```")
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n"), nil
}

// ShouldReturnResult indicates whether the tool should return its result to the LLM
func (t *GitHubSearch) ShouldReturnResult() bool {
	return true
}

// indexRepository indexes a repository in the vector store
func (t *GitHubSearch) indexRepository(ctx context.Context, repo string, contentTypes []github.ContentType) error {
	// Fetch repository content
	result, err := t.ghClient.GetRepositoryContent(ctx, &github.SearchQuery{
		Repo:         repo,
		ContentTypes: contentTypes,
		Limit:        100, // Limit to 100 items for now
	})
	if err != nil {
		return fmt.Errorf("failed to fetch repository content: %w", err)
	}

	// Index each item
	for _, item := range result.Items {
		// Generate embedding
		embedding, err := t.embeddings.GenerateEmbedding(ctx, item.Content)
		if err != nil {
			// Skip items that can't be embedded
			continue
		}

		// Add to vector store
		err = t.vectorStore.Add(ctx, item.Path, item.Content, embedding, item.Metadata)
		if err != nil {
			// Skip items that can't be added
			continue
		}
	}

	return nil
}

// AddRepository adds a repository to the vector store
func (t *GitHubSearch) AddRepository(ctx context.Context, repo string, contentTypes []string) error {
	// Convert string content types to ContentType
	var ghContentTypes []github.ContentType
	for _, ct := range contentTypes {
		switch ct {
		case "code":
			ghContentTypes = append(ghContentTypes, github.ContentTypeCode)
		case "repo":
			ghContentTypes = append(ghContentTypes, github.ContentTypeRepo)
		case "pr":
			ghContentTypes = append(ghContentTypes, github.ContentTypePR)
		case "issue":
			ghContentTypes = append(ghContentTypes, github.ContentTypeIssue)
		}
	}

	return t.indexRepository(ctx, repo, ghContentTypes)
}
