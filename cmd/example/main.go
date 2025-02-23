package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/getstingrai/agents"
	"github.com/getstingrai/agents/llm"
	"github.com/getstingrai/agents/providers/anthropic"
	"github.com/getstingrai/agents/providers/groq"
	"github.com/getstingrai/agents/providers/openai"
	"github.com/getstingrai/agents/tools"
	"github.com/getstingrai/agents/tools/google"
)

func main() {
	var providerName, modelName string
	flag.StringVar(&providerName, "provider", "anthropic", "provider to use")
	flag.StringVar(&modelName, "model", "", "model to use")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var provider llm.LLM
	switch providerName {
	case "anthropic":
		provider = anthropic.New()
	case "openai":
		provider = openai.New()
	case "groq":
		provider = groq.New()
	}

	googleClient, err := google.New()
	if err != nil {
		log.Fatal(err)
	}

	a := agents.NewStandardAgent(agents.StandardAgentSpec{
		Name:         "test",
		Role:         &agents.Role{Name: "test"},
		LLM:          provider,
		Tools:        []llm.Tool{tools.NewGoogleSearch(googleClient)},
		CacheControl: "ephemeral",
		LogLevel:     "info",
		Hooks: llm.Hooks{
			llm.BeforeGenerate: func(ctx context.Context, hookCtx *llm.HookContext) {
				fmt.Println("before generate")
				for _, message := range hookCtx.Messages {
					fmt.Println("message", message.Role)
					for i, content := range message.Content {
						fmt.Printf("  content %d: %s %s\n", i, content.Type, content.Text)
					}
				}
			},
			llm.AfterGenerate: func(ctx context.Context, hookCtx *llm.HookContext) {
				fmt.Println("after generate")
				for _, content := range hookCtx.Response.Message().Content {
					fmt.Println("content", content.Type, content.Text)
				}
			},
		},
	})

	if err := a.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer a.Stop(ctx)

	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter a task description: ")
		description, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		description = strings.TrimSpace(description)

		task := agents.NewTask(agents.TaskSpec{Description: description})

		promise, err := a.Work(ctx, task)
		if err != nil {
			log.Fatal(err)
		}
		result, err := promise.Get(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(result.Output.Content)
	}
}
