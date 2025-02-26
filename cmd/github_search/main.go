package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/tools"
)

func main() {
	// Get GitHub token from environment
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}

	// Get Anthropic API key from environment
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		log.Fatal("ANTHROPIC_API_KEY environment variable is required")
	}

	// Create a GitHub search tool
	// For a fixed repository:
	// githubSearchTool := tools.NewFixedRepoGitHubSearch(ghToken, "getstingrai/dive", 10, []string{"code", "repo"})

	// For dynamic repository search:
	githubSearchTool := tools.NewGitHubSearch(ghToken, 10)

	// Create an LLM provider (using Anthropic Claude)
	llmProvider := anthropic.New(
		anthropic.WithAPIKey(anthropicKey),
		anthropic.WithModel("claude-3-7-sonnet-20250219"),
	)

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

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start the agent
	agent.Start(ctx)

	// Send a message to the agent
	fmt.Println("Sending message to agent...")
	message := llm.NewUserMessage("Search for 'search tool' in the 'crewAIInc/crewAI-tools' repository")
	response, err := agent.Chat(ctx, message)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	// Print the response
	fmt.Println("Agent response:")
	fmt.Println(response.Message().Text())

	// Example of searching in a different repository
	fmt.Println("\nSearching in a different repository...")
	message = llm.NewUserMessage("Search for 'embeddings' in the 'openai/openai-cookbook' repository")
	response, err = agent.Chat(ctx, message)
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	// Print the response
	fmt.Println("Agent response:")
	fmt.Println(response.Message().Text())

	// Stop the agent
	agent.Stop(ctx)
}
