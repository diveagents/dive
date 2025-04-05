package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/agent"
	"github.com/diveagents/dive/config"
	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/slogger"
	"github.com/diveagents/dive/toolkit"
	"github.com/diveagents/dive/toolkit/firecrawl"
	"github.com/diveagents/dive/toolkit/google"
)

func main() {
	var verbose bool
	var providerName, modelName string
	flag.StringVar(&providerName, "provider", "anthropic", "provider to use")
	flag.StringVar(&modelName, "model", "", "model to use")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.Parse()

	ctx := context.Background()

	logger := slogger.New(slogger.LevelDebug)

	model, err := config.GetModel(providerName, modelName)
	if err != nil {
		log.Fatal(err)
	}

	var tools []llm.Tool

	if key := os.Getenv("FIRECRAWL_API_KEY"); key != "" {
		client, err := firecrawl.New(firecrawl.WithAPIKey(key))
		if err != nil {
			log.Fatal(err)
		}
		scraper := toolkit.NewFetchTool(client)
		tools = append(tools, scraper)

		logger.Info("firecrawl enabled")
	}

	if key := os.Getenv("GOOGLE_SEARCH_CX"); key != "" {
		googleClient, err := google.New()
		if err != nil {
			log.Fatal(err)
		}
		tools = append(tools, toolkit.NewSearchTool(googleClient))

		logger.Info("google search enabled")
	}

	a, err := agent.New(agent.Options{
		Name:   "Research Assistant",
		Model:  model,
		Tools:  tools,
		Logger: logger,
	})
	if err != nil {
		log.Fatal(err)
	}

	task := agent.NewTask(agent.TaskOptions{
		Name: "Research the history of beer",
		Prompt: &dive.Prompt{
			Text:         "Briefly research the history of beer",
			Output:       "The history of beer",
			OutputFormat: dive.OutputFormatMarkdown,
		},
	})

	prompt, err := task.Prompt()
	if err != nil {
		log.Fatal(err)
	}

	stream, err := a.StreamResponse(ctx, dive.WithInput(prompt.Text))
	if err != nil {
		log.Fatal(err)
	}
	defer stream.Close()

	for stream.Next(ctx) {
		event := stream.Event()
		if event.Type == dive.EventTypeResponseCompleted {
			response := event.Response
			for _, item := range response.Items {
				if item.Type == dive.ResponseItemTypeMessage {
					for _, block := range item.Message.Content {
						fmt.Println(block.Text)
					}
				}
			}
		}
	}
}
