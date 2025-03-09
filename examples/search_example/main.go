package main

import (
	"context"
	"log"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/tools"
	"github.com/getstingrai/dive/tools/google"
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

	agent := dive.NewAgent(dive.AgentOptions{
		Name:         "Chris",
		Description:  "Research Assistant",
		Instructions: "Use Google to research assigned topics",
		LLM:          anthropic.New(),
		IsSupervisor: false,
		LogLevel:     "debug",
		Logger:       slogger.New(slogger.LevelDebug),
		Tools:        []llm.Tool{tools.NewGoogleSearch(googleClient)},
	})
	if err := agent.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer agent.Stop(ctx)

	step := dive.NewStep(dive.StepOptions{
		Description: "Research the history of computing and summarize in 3 paragraphs",
	})

	stream, err := agent.Work(ctx, step)
	if err != nil {
		log.Fatal(err)
	}

	var result *dive.StepResult
	for event := range stream.Channel() {
		if event.Error != "" {
			log.Fatal(event.Error)
		}
		if event.StepResult != nil {
			result = event.StepResult
			break
		}
	}

	if result != nil {
		log.Printf("Step result: %s", result.Content)
	} else {
		log.Fatal("No result found")
	}
}
