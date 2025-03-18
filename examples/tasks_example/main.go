package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/providers/groq"
	"github.com/getstingrai/dive/providers/openai"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/tools"
	"github.com/getstingrai/dive/tools/google"
	"github.com/getstingrai/dive/workflow"
	"github.com/mendableai/firecrawl-go"
)

func main() {
	var verbose bool
	var providerName string
	flag.StringVar(&providerName, "provider", "anthropic", "provider to use")
	flag.BoolVar(&verbose, "verbose", false, "verbose output")
	flag.Parse()

	ctx := context.Background()

	var provider llm.LLM
	switch providerName {
	case "anthropic":
		provider = anthropic.New()
	case "openai":
		provider = openai.New()
	case "groq":
		provider = groq.New(groq.WithModel("deepseek-r1-distill-llama-70b"))
	}

	var theTools []llm.Tool

	if key := os.Getenv("FIRECRAWL_API_KEY"); key != "" {
		app, err := firecrawl.NewFirecrawlApp(key, "")
		if err != nil {
			log.Fatal(err)
		}
		scraper := tools.NewFirecrawlScrapeTool(tools.FirecrawlScrapeToolOptions{
			App: app,
		})
		theTools = append(theTools, scraper)

		log.Println("firecrawl enabled")
	}

	if key := os.Getenv("GOOGLE_SEARCH_CX"); key != "" {
		googleClient, err := google.New()
		if err != nil {
			log.Fatal(err)
		}
		theTools = append(theTools, tools.NewGoogleSearch(googleClient))

		log.Println("google search enabled")
	}

	a := agent.NewAgent(agent.AgentOptions{
		Name:         "Research Assistant",
		CacheControl: "ephemeral",
		LLM:          provider,
		Tools:        theTools,
		LogLevel:     "debug",
		Logger:       slogger.New(slogger.LevelDebug),
	})

	if err := a.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer a.Stop(ctx)

	task := workflow.NewTask(workflow.TaskOptions{
		Name:        "Research the history of the internet",
		Description: "Research the history of the internet",
		Outputs: map[string]dive.Output{
			"history": {
				Name:        "history",
				Description: "The history of the internet",
				Format:      string(dive.OutputMarkdown),
			},
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
