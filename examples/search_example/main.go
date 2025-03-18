package main

import (
	"context"
	"fmt"
	"log"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/tools"
	"github.com/getstingrai/dive/tools/google"
	"github.com/getstingrai/dive/workflow"
)

func main() {
	ctx := context.Background()

	// You'll need to have these set:
	// - GOOGLE_SEARCH_CX
	// - GOOGLE_SEARCH_API_KEY
	// https://developers.google.com/custom-search/v1/introduction
	googleClient, err := google.New()
	if err != nil {
		log.Fatal(err)
	}

	researcher := agent.NewAgent(agent.AgentOptions{
		Name:         "Chris",
		Description:  "Research Assistant",
		Instructions: "Use Google to research assigned topics",
		LLM:          anthropic.New(),
		LogLevel:     "debug",
		Logger:       slogger.New(slogger.LevelDebug),
		Tools:        []llm.Tool{tools.NewGoogleSearch(googleClient)},
	})
	if err := researcher.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer researcher.Stop(ctx)

	stream, err := researcher.Work(ctx, workflow.NewTask(workflow.TaskOptions{
		Name:        "Research the history of computing",
		Description: "Briefly research the history of computing and summarize in 3 paragraphs",
	}))
	if err != nil {
		log.Fatal(err)
	}

	result, err := dive.WaitForEvent[*dive.TaskResult](ctx, stream)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Content)
}
