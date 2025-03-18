package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/providers/groq"
	"github.com/getstingrai/dive/providers/openai"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/toolkit"
	"github.com/getstingrai/dive/toolkit/google"
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
		provider = anthropic.New()
	case "openai":
		provider = openai.New()
	case "groq":
		provider = groq.New(groq.WithModel("deepseek-r1-distill-llama-70b"))
	}

	googleClient, err := google.New()
	if err != nil {
		log.Fatal(err)
	}

	logger := slogger.New(slogger.LevelInfo)

	a := agent.NewAgent(agent.AgentOptions{
		Name: "Dr. Smith",
		Description: `
You are a virtual doctor for role-playing purposes only. You can discuss general
medical topics, symptoms, and health advice, but always clarify that you're not
a real doctor and cannot provide actual medical diagnosis or treatment. Refuse
to answer non-medical questions. Use maximum medical jargon.`,
		LLM:          provider,
		Tools:        []llm.Tool{toolkit.NewGoogleSearch(googleClient)},
		CacheControl: "ephemeral",
		LogLevel:     "info",
		Logger:       logger,
	})

	if err := a.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer a.Stop(ctx)

	for {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter a chat message about a medical topic: ")
		message, err := reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		message = strings.TrimSpace(message)
		if message == "exit" {
			break
		}
		if message == "" {
			continue
		}

		iterator, err := a.Stream(ctx, llm.NewUserMessage(message))
		if err != nil {
			log.Fatal(err)
		}
		defer iterator.Close()

		var inToolUse bool
		toolUseAccum := ""
		toolName := ""
		toolID := ""

		for iterator.Next(ctx) {
			event := iterator.Event()
			switch payload := event.Payload.(type) {
			case *llm.Event:
				if payload.ContentBlock != nil {
					cb := payload.ContentBlock
					if cb.Type == "tool_use" {
						toolName = cb.Name
						toolID = cb.ID
					}
				}
				if payload.Delta != nil {
					delta := payload.Delta
					if delta.PartialJSON != "" {
						if !inToolUse {
							inToolUse = true
							fmt.Println("\n----")
						}
						toolUseAccum += delta.PartialJSON
					} else if delta.Text != "" {
						if inToolUse {
							fmt.Println("NAME:", toolName, "ID:", toolID)
							fmt.Println(toolUseAccum)
							fmt.Println("----")
							inToolUse = false
							toolUseAccum = ""
						}
						fmt.Print(delta.Text)
					}
				}
			}
		}
		fmt.Println()
	}
}
