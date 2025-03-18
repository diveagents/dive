package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/environment"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/providers/groq"
	"github.com/getstingrai/dive/providers/openai"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/toolkit"
	"github.com/getstingrai/dive/toolkit/google"
	"github.com/getstingrai/dive/workflow"
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

	var provider llm.LLM
	switch providerName {
	case "anthropic":
		var opts []anthropic.Option
		if modelName != "" {
			opts = append(opts, anthropic.WithModel(modelName))
		}
		provider = anthropic.New(opts...)
	case "openai":
		var opts []openai.Option
		if modelName != "" {
			opts = append(opts, openai.WithModel(modelName))
		}
		provider = openai.New(opts...)
	case "groq":
		var opts []groq.Option
		if modelName != "" {
			opts = append(opts, groq.WithModel(modelName))
		}
		provider = groq.New(opts...)
	}

	logLevel := "info"
	if verbose {
		logLevel = "debug"
	}
	logger := slogger.New(slogger.LevelFromString(logLevel))

	var theTools []llm.Tool

	if key := os.Getenv("FIRECRAWL_API_KEY"); key != "" {
		app, err := firecrawl.NewFirecrawlApp(key, "")
		if err != nil {
			log.Fatal(err)
		}
		scraper := toolkit.NewFirecrawlScrapeTool(toolkit.FirecrawlScrapeToolOptions{
			App: app,
		})
		theTools = append(theTools, scraper)
		logger.Info("firecrawl enabled")
	} else {
		logger.Info("firecrawl is not enabled")
	}

	if key := os.Getenv("GOOGLE_SEARCH_CX"); key != "" {
		googleClient, err := google.New()
		if err != nil {
			log.Fatal(err)
		}
		theTools = append(theTools, toolkit.NewGoogleSearch(googleClient))
		logger.Info("google search enabled")
	} else {
		logger.Info("google search is not enabled")
	}

	if len(theTools) == 0 {
		logger.Warn("no tools enabled")
	}

	supervisor := agent.NewAgent(agent.AgentOptions{
		Name:         "Supervisor",
		Description:  "Research Supervisor and Renowned Author. Assign research tasks to the research assistant, but prepare the final reports or biographies yourself.",
		IsSupervisor: true,
		Subordinates: []string{"Research Assistant"},
		CacheControl: "ephemeral",
		LLM:          provider,
		LogLevel:     logLevel,
		Logger:       logger,
	})

	researcher := agent.NewAgent(agent.AgentOptions{
		Name:         "Research Assistant",
		Description:  "You are an expert research assistant. Don't go too deep into the details unless specifically asked.",
		CacheControl: "ephemeral",
		LLM:          provider,
		Tools:        theTools,
		LogLevel:     logLevel,
		Logger:       logger,
	})

	w, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name:        "Research Workflow",
		Description: "A workflow for the research assistant. The supervisor will assign tasks to the research assistant.",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name: "Research Step",
				Task: workflow.NewTask(workflow.TaskOptions{
					Name:        "Research Task",
					Description: "Research the history of maple syrup production in Vermont.",
					Agent:       researcher,
				}),
			}),
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	env, err := environment.New(environment.EnvironmentOptions{
		Name:        "Research Environment",
		Description: "A research environment for the research assistant. The supervisor will assign tasks to the research assistant.",
		Agents: []dive.Agent{
			supervisor,
			researcher,
		},
		Workflows: []*workflow.Workflow{w},
	})
	if err != nil {
		log.Fatal(err)
	}

	execution, err := env.StartWorkflow(ctx, w, map[string]interface{}{})
	if err != nil {
		log.Fatal(err)
	}

	if err := execution.Wait(); err != nil {
		log.Fatal(err)
	}

	for stepName, output := range execution.StepOutputs() {
		fmt.Printf("---- step %s ----\n", stepName)
		fmt.Println(output)
		fmt.Println()
	}
}
