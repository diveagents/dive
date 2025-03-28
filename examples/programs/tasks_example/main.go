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
	"github.com/diveagents/dive/toolkit/google"
	"github.com/mendableai/firecrawl-go"
)

func main() {
	var verbose bool
	var providerName, modelName string
	flag.StringVar(&providerName, "provider", "anthropic", "provider to use")
	flag.StringVar(&modelName, "model", "", "model to use")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.Parse()

	ctx := context.Background()

	model, err := config.GetModel(providerName, modelName)
	if err != nil {
		log.Fatal(err)
	}

	var tools []llm.Tool

	if key := os.Getenv("FIRECRAWL_API_KEY"); key != "" {
		app, err := firecrawl.NewFirecrawlApp(key, "")
		if err != nil {
			log.Fatal(err)
		}
		scraper := toolkit.NewFirecrawlScrapeTool(toolkit.FirecrawlScrapeToolOptions{
			App: app,
		})
		tools = append(tools, scraper)

		log.Println("firecrawl enabled")
	}

	if key := os.Getenv("GOOGLE_SEARCH_CX"); key != "" {
		googleClient, err := google.New()
		if err != nil {
			log.Fatal(err)
		}
		tools = append(tools, toolkit.NewGoogleSearch(googleClient))

		log.Println("google search enabled")
	}

	logger := slogger.New(slogger.LevelDebug)

	a, err := agent.New(agent.Options{
		Name:      "Research Assistant",
		Model:     model,
		Tools:     tools,
		Logger:    logger,
		AutoStart: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Stop(ctx)

	task := agent.NewTask(agent.TaskOptions{
		Name: "Research the history of beer",
		Prompt: &dive.Prompt{
			Text:         "Research the history of beer",
			Output:       "The history of beer",
			OutputFormat: dive.OutputFormatMarkdown,
		},
	})

	iterator, err := a.Work(ctx, task)
	if err != nil {
		log.Fatal(err)
	}
	defer iterator.Close()

	for iterator.Next(ctx) {
		event := iterator.Event()
		switch p := event.Payload.(type) {
		case *dive.TaskResult:
			fmt.Println("result:\n", p.Content)
		}
	}
}
