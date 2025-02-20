package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getstingrai/agents/llm"
	"github.com/getstingrai/agents/tools/google"
)

var _ llm.Tool = &GoogleSearch{}

type GoogleSearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type GoogleSearch struct {
	client *google.Client
}

func NewGoogleSearch(client *google.Client) *GoogleSearch {
	return &GoogleSearch{client: client}
}

func (t *GoogleSearch) Name() string {
	return "GoogleSearch"
}

func (t *GoogleSearch) Description() string {
	return "Returns search results from Google for the given search query. The response includes the url, title, and description of each webpage in the search results."
}

func (t *GoogleSearch) Definition() *llm.ToolDefinition {
	return &llm.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: llm.Schema{
			Type:     "object",
			Required: []string{"query"},
			Properties: map[string]*llm.SchemaProperty{
				"query": {
					Type:        "string",
					Description: "The search query, e.g. 'cloud security companies'",
				},
				"limit": {
					Type:        "number",
					Description: "The maximum number of results to return (Default: 10, Max: 30)",
				},
			},
		},
	}
}

func (t *GoogleSearch) Call(ctx context.Context, input json.RawMessage) (string, error) {
	var s GoogleSearchInput
	if err := json.Unmarshal(input, &s); err != nil {
		return "", err
	}
	if s.Limit <= 0 {
		s.Limit = 10
	}
	if s.Limit > 30 {
		s.Limit = 30
	}
	results, err := t.client.Search(ctx, &google.Query{
		Text:  s.Query,
		Limit: s.Limit,
	})
	if err != nil {
		return "", err
	}
	if len(results.Items) == 0 {
		return "ERROR: no results found", nil
	}
	if len(results.Items) > s.Limit {
		results.Items = results.Items[:s.Limit]
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("Found %d search results:", len(results.Items)))
	lines = append(lines, "")

	for i, r := range results.Items {
		lines = append(lines, fmt.Sprintf("## Search Result %d", i+1))
		lines = append(lines, fmt.Sprintf("- Title: %s", r.Title))
		lines = append(lines, fmt.Sprintf("- URL: %s", r.URL))
		lines = append(lines, fmt.Sprintf("- Description: %s", r.Description))
		lines = append(lines, "")
	}

	text := strings.Join(lines, "\n")
	text = strings.ReplaceAll(text, "```", "")
	return text, nil
}
