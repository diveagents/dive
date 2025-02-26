package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a GitHub API client
type Client struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

// Option is a function that configures a Client
type Option func(*Client)

// WithBaseURL sets the base URL for the GitHub API
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient sets the HTTP client for the GitHub API
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// New creates a new GitHub client
func New(token string, opts ...Option) *Client {
	client := &Client{
		token:   token,
		baseURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// ContentType represents the type of content to search for
type ContentType string

const (
	ContentTypeCode  ContentType = "code"
	ContentTypeRepo  ContentType = "repo"
	ContentTypePR    ContentType = "pr"
	ContentTypeIssue ContentType = "issue"
)

// RepoContent represents content from a GitHub repository
type RepoContent struct {
	Path     string
	Type     string
	Content  string
	URL      string
	Metadata map[string]interface{}
}

// SearchQuery represents a search query for GitHub
type SearchQuery struct {
	Repo         string
	ContentTypes []ContentType
	Limit        int
}

// SearchResult represents a search result from GitHub
type SearchResult struct {
	Items []RepoContent
}

// GetRepositoryContent fetches content from a GitHub repository
func (c *Client) GetRepositoryContent(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	if query.Limit <= 0 {
		query.Limit = 100
	}

	result := &SearchResult{
		Items: []RepoContent{},
	}

	// Process each content type
	for _, contentType := range query.ContentTypes {
		switch contentType {
		case ContentTypeCode:
			// Fetch code files
			files, err := c.getRepositoryFiles(ctx, query.Repo)
			if err != nil {
				return nil, err
			}
			result.Items = append(result.Items, files...)
		case ContentTypeRepo:
			// Fetch repository info
			repoInfo, err := c.getRepositoryInfo(ctx, query.Repo)
			if err != nil {
				return nil, err
			}
			result.Items = append(result.Items, repoInfo)
		case ContentTypePR:
			// Fetch pull requests
			prs, err := c.getPullRequests(ctx, query.Repo, query.Limit)
			if err != nil {
				return nil, err
			}
			result.Items = append(result.Items, prs...)
		case ContentTypeIssue:
			// Fetch issues
			issues, err := c.getIssues(ctx, query.Repo, query.Limit)
			if err != nil {
				return nil, err
			}
			result.Items = append(result.Items, issues...)
		}

		// Limit the total number of items
		if len(result.Items) >= query.Limit {
			result.Items = result.Items[:query.Limit]
			break
		}
	}

	return result, nil
}

// getRepositoryFiles fetches files from a GitHub repository
func (c *Client) getRepositoryFiles(ctx context.Context, repo string) ([]RepoContent, error) {
	// This is a simplified implementation that would need to be expanded
	// to recursively fetch files from the repository
	apiURL := fmt.Sprintf("%s/repos/%s/contents", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error from GitHub API: %s", string(body))
	}

	var contents []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Type        string `json:"type"`
		DownloadURL string `json:"download_url"`
		URL         string `json:"url"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	var result []RepoContent

	// Process each content item
	for _, item := range contents {
		// Skip directories for now
		if item.Type == "dir" {
			continue
		}

		// Fetch the file content
		content, err := c.getFileContent(ctx, item.DownloadURL)
		if err != nil {
			// Skip files that can't be fetched
			continue
		}

		result = append(result, RepoContent{
			Path:    item.Path,
			Type:    "code",
			Content: content,
			URL:     item.URL,
			Metadata: map[string]interface{}{
				"path": item.Path,
				"type": "code",
				"name": item.Name,
			},
		})
	}

	return result, nil
}

// getFileContent fetches the content of a file
func (c *Client) getFileContent(ctx context.Context, downloadURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching file: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response: %w", err)
	}

	return string(body), nil
}

// getRepositoryInfo fetches information about a GitHub repository
func (c *Client) getRepositoryInfo(ctx context.Context, repo string) (RepoContent, error) {
	apiURL := fmt.Sprintf("%s/repos/%s", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return RepoContent{}, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return RepoContent{}, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return RepoContent{}, fmt.Errorf("error from GitHub API: %s", string(body))
	}

	var repoInfo struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		URL         string `json:"html_url"`
		Stars       int    `json:"stargazers_count"`
		Forks       int    `json:"forks_count"`
		OpenIssues  int    `json:"open_issues_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return RepoContent{}, fmt.Errorf("error decoding response: %w", err)
	}

	content := fmt.Sprintf("Repository: %s\nDescription: %s\nStars: %d\nForks: %d\nOpen Issues: %d\nURL: %s",
		repoInfo.FullName, repoInfo.Description, repoInfo.Stars, repoInfo.Forks, repoInfo.OpenIssues, repoInfo.URL)

	return RepoContent{
		Path:    "",
		Type:    "repo",
		Content: content,
		URL:     repoInfo.URL,
		Metadata: map[string]interface{}{
			"type":        "repo",
			"name":        repoInfo.Name,
			"full_name":   repoInfo.FullName,
			"description": repoInfo.Description,
			"stars":       repoInfo.Stars,
			"forks":       repoInfo.Forks,
			"open_issues": repoInfo.OpenIssues,
		},
	}, nil
}

// getPullRequests fetches pull requests from a GitHub repository
func (c *Client) getPullRequests(ctx context.Context, repo string, limit int) ([]RepoContent, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/pulls?state=all&per_page=%d", c.baseURL, repo, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error from GitHub API: %s", string(body))
	}

	var prs []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		Body      string `json:"body"`
		URL       string `json:"html_url"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	var result []RepoContent

	for _, pr := range prs {
		content := fmt.Sprintf("PR #%d: %s\nState: %s\nAuthor: %s\nCreated: %s\nUpdated: %s\nURL: %s\n\n%s",
			pr.Number, pr.Title, pr.State, pr.User.Login, pr.CreatedAt, pr.UpdatedAt, pr.URL, pr.Body)

		result = append(result, RepoContent{
			Path:    fmt.Sprintf("pull/%d", pr.Number),
			Type:    "pr",
			Content: content,
			URL:     pr.URL,
			Metadata: map[string]interface{}{
				"type":       "pr",
				"number":     pr.Number,
				"title":      pr.Title,
				"state":      pr.State,
				"author":     pr.User.Login,
				"created_at": pr.CreatedAt,
				"updated_at": pr.UpdatedAt,
			},
		})
	}

	return result, nil
}

// getIssues fetches issues from a GitHub repository
func (c *Client) getIssues(ctx context.Context, repo string, limit int) ([]RepoContent, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/issues?state=all&per_page=%d", c.baseURL, repo, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error from GitHub API: %s", string(body))
	}

	var issues []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		Body      string `json:"body"`
		URL       string `json:"html_url"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	var result []RepoContent

	for _, issue := range issues {
		// Skip pull requests (GitHub API returns PRs as issues)
		if strings.Contains(issue.URL, "/pull/") {
			continue
		}

		content := fmt.Sprintf("Issue #%d: %s\nState: %s\nAuthor: %s\nCreated: %s\nUpdated: %s\nURL: %s\n\n%s",
			issue.Number, issue.Title, issue.State, issue.User.Login, issue.CreatedAt, issue.UpdatedAt, issue.URL, issue.Body)

		result = append(result, RepoContent{
			Path:    fmt.Sprintf("issues/%d", issue.Number),
			Type:    "issue",
			Content: content,
			URL:     issue.URL,
			Metadata: map[string]interface{}{
				"type":       "issue",
				"number":     issue.Number,
				"title":      issue.Title,
				"state":      issue.State,
				"author":     issue.User.Login,
				"created_at": issue.CreatedAt,
				"updated_at": issue.UpdatedAt,
			},
		})
	}

	return result, nil
}

// SearchCode searches for code in a GitHub repository
func (c *Client) SearchCode(ctx context.Context, query string, repo string, limit int) ([]RepoContent, error) {
	// URL encode the query
	encodedQuery := url.QueryEscape(fmt.Sprintf("%s repo:%s", query, repo))
	apiURL := fmt.Sprintf("%s/search/code?q=%s&per_page=%d", c.baseURL, encodedQuery, limit)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("error from GitHub API: %s", string(body))
	}

	var searchResult struct {
		TotalCount int `json:"total_count"`
		Items      []struct {
			Name       string `json:"name"`
			Path       string `json:"path"`
			HTMLURL    string `json:"html_url"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			URL string `json:"url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	var result []RepoContent

	for _, item := range searchResult.Items {
		// Fetch the file content
		content, err := c.getFileContent(ctx, item.URL)
		if err != nil {
			// Skip files that can't be fetched
			continue
		}

		result = append(result, RepoContent{
			Path:    item.Path,
			Type:    "code",
			Content: content,
			URL:     item.HTMLURL,
			Metadata: map[string]interface{}{
				"path":       item.Path,
				"type":       "code",
				"name":       item.Name,
				"repository": item.Repository.FullName,
			},
		})
	}

	return result, nil
}
