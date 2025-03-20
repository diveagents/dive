package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/slogger"
	"github.com/stretchr/testify/require"
)

func TestAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	agent := NewAgent(AgentOptions{
		Name:         "Testing Agent",
		Description:  "Test",
		Instructions: "",
		IsSupervisor: false,
		LLM:          anthropic.New(),
	})

	err := agent.Start(ctx)
	require.NoError(t, err)

	err = agent.Stop(ctx)
	require.NoError(t, err)
}

func TestAgentChat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	agent := NewAgent(AgentOptions{
		Name: "Testing Agent",
		LLM:  anthropic.New(),
	})

	err := agent.Start(ctx)
	require.NoError(t, err)

	response, err := agent.Generate(ctx, llm.NewUserMessage("Hello, world!"))
	require.NoError(t, err)

	text := strings.ToLower(response.Message().Text())
	matches := strings.Contains(text, "hello") || strings.Contains(text, "hi")
	require.True(t, matches)

	err = agent.Stop(ctx)
	require.NoError(t, err)
}

func TestAgentChatWithTools(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	echoToolDef := llm.ToolDefinition{
		Name:        "echo",
		Description: "Echoes back the input",
		Parameters: llm.Schema{
			Type:     "object",
			Required: []string{"text"},
			Properties: map[string]*llm.SchemaProperty{
				"text": {Type: "string", Description: "The text to echo back"},
			},
		},
	}

	var echoInput string

	echoFunc := func(ctx context.Context, input string) (string, error) {
		var m map[string]interface{}
		err := json.Unmarshal([]byte(input), &m)
		require.NoError(t, err)
		echoInput = m["text"].(string)
		return input, nil
	}

	agent := NewAgent(AgentOptions{
		LLM:   anthropic.New(),
		Tools: []llm.Tool{llm.NewTool(&echoToolDef, echoFunc)},
	})

	err := agent.Start(ctx)
	require.NoError(t, err)

	response, err := agent.Generate(ctx, llm.NewUserMessage("Please use the echo tool to echo 'hello world'"))
	require.NoError(t, err)

	text := strings.ToLower(response.Message().Text())
	require.Contains(t, text, "echo")
	require.Equal(t, "hello world", echoInput)

	err = agent.Stop(ctx)
	require.NoError(t, err)
}

func TestAgentTask(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	logger := slogger.New(slogger.LevelDebug)

	agent := NewAgent(AgentOptions{
		Name:        "Poet",
		Description: "You're a poet that loves writing limericks",
		LLM:         anthropic.New(),
		Logger:      logger,
	})

	require.NoError(t, agent.Start(ctx))
	defer agent.Stop(ctx)

	stream, err := agent.Work(ctx, &SimpleTask{
		name:        "Limerick",
		description: "Write a limerick about a cat",
		outputs: map[string]dive.Output{
			"poem": {
				Name:        "poem",
				Type:        "string",
				Description: "A limerick about a cat",
			},
		},
	}, map[string]any{})
	require.NoError(t, err)

	event, err := dive.WaitForEvent[*dive.TaskResult](ctx, stream)
	require.NoError(t, err)

	content := strings.ToLower(event.Content)
	matches := 0
	for _, word := range []string{"cat", "whiskers", "paws"} {
		if strings.Contains(content, word) {
			matches += 1
		}
	}
	require.Greater(t, matches, 0, "poem should contain at least one cat-related word")
}

func TestAgentChatSystemPrompt(t *testing.T) {
	agent := NewAgent(AgentOptions{
		Name:         "TestAgent",
		Description:  "You are a research assistant.",
		Instructions: "You are extremely thorough and detail-oriented.",
		IsSupervisor: false,
		LLM:          anthropic.New(),
	})

	// Get the chat system prompt
	chatSystemPrompt, err := agent.getSystemPromptForMode("chat")
	require.NoError(t, err)

	// Verify that the chat system prompt doesn't contain the status section
	require.NotContains(t, chatSystemPrompt, "<status>")
	require.NotContains(t, chatSystemPrompt, "active")
	require.NotContains(t, chatSystemPrompt, "completed")
	require.NotContains(t, chatSystemPrompt, "paused")
	require.NotContains(t, chatSystemPrompt, "blocked")
	require.NotContains(t, chatSystemPrompt, "error")

	// Get the task system prompt
	taskSystemPrompt, err := agent.getSystemPromptForMode("task")
	require.NoError(t, err)

	// Verify that the task system prompt contains the status section
	require.Contains(t, taskSystemPrompt, "<status>")
	require.Contains(t, taskSystemPrompt, "active")
	require.Contains(t, taskSystemPrompt, "completed")
	require.Contains(t, taskSystemPrompt, "paused")
	require.Contains(t, taskSystemPrompt, "blocked")
	require.Contains(t, taskSystemPrompt, "error")
}
