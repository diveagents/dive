package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/toolkit"
	"github.com/getstingrai/dive/toolkit/google"
	"github.com/getstingrai/dive/workflow"
)

func main() {

	var logLevel string
	if os.Getenv("LOG_LEVEL") != "" {
		logLevel = os.Getenv("LOG_LEVEL")
	} else {
		logLevel = "info"
	}

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
		Logger:       slogger.New(slogger.LevelFromString(logLevel)),
		Tools:        []llm.Tool{toolkit.NewGoogleSearch(googleClient)},
	})
	if err := researcher.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer researcher.Stop(ctx)

	task := workflow.NewTask(workflow.TaskOptions{
		Name:        "Research the history of computing",
		Description: "Briefly research the history of computing and summarize in 3 paragraphs",
	})

	stream, err := researcher.Work(ctx, task, nil)
	if err != nil {
		log.Fatal(err)
	}

	result, err := dive.WaitForEvent[*dive.TaskResult](ctx, stream)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Content)
}
