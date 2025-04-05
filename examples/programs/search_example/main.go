package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/agent"
	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/llm/providers/anthropic"
	"github.com/diveagents/dive/slogger"
	"github.com/diveagents/dive/toolkit"
	"github.com/diveagents/dive/toolkit/google"
)

func main() {

	var logLevel string
	if os.Getenv("LOG_LEVEL") != "" {
		logLevel = os.Getenv("LOG_LEVEL")
	} else {
		logLevel = "debug"
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

	researcher, err := agent.New(agent.Options{
		Name:   "Research Assistant",
		Goal:   "Use Google to research assigned topics",
		Model:  anthropic.New(),
		Logger: slogger.New(slogger.LevelFromString(logLevel)),
		Tools:  []llm.Tool{toolkit.NewSearchTool(googleClient)},
	})
	if err != nil {
		log.Fatal(err)
	}

	task := agent.NewTask(agent.TaskOptions{
		Timeout: 10 * time.Second,
		Prompt: &dive.Prompt{
			Text:         "Research the history of computing",
			Output:       "A brief report",
			OutputFormat: "markdown",
		},
	})

	prompt, err := task.Prompt()
	if err != nil {
		log.Fatal(err)
	}

	response, err := researcher.CreateResponse(ctx,
		dive.WithInput(prompt.Text),
	)
	if err != nil {
		log.Fatal(err)
	}

	for _, item := range response.Items {
		if item.Type == dive.ResponseItemTypeMessage {
			for _, block := range item.Message.Content {
				if block.Type == llm.ContentTypeText {
					fmt.Println(block.Text)
				}
			}
		}
	}
}
