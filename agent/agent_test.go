package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/llm/providers/anthropic"
	"github.com/stretchr/testify/require"
)

func TestAgent(t *testing.T) {
	agent, err := New(Options{
		Name:         "Testing Agent",
		Goal:         "Test the agent",
		Backstory:    "You are a testing agent",
		IsSupervisor: false,
		Model:        anthropic.New(),
	})
	require.NoError(t, err)
	require.NotNil(t, agent)
}

func TestAgentChat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	agent, err := New(Options{
		Name:  "Testing Agent",
		Model: anthropic.New(),
	})
	require.NoError(t, err)

	stream, err := agent.StreamResponse(ctx, dive.WithInput("Hello, world!"))
	require.NoError(t, err)

	generations, err := dive.ReadEventPayloads[*dive.Generation](ctx, stream)
	require.NoError(t, err)
	require.Greater(t, len(generations), 0)

	lastGeneration := generations[len(generations)-1]
	lastMessage, ok := lastGeneration.LastMessage()
	require.True(t, ok)

	text := strings.ToLower(lastMessage.Text())
	matches := strings.Contains(text, "hello") || strings.Contains(text, "hi")
	require.True(t, matches)
}

// func TestAgentChatWithTools(t *testing.T) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
// 	defer cancel()

// 	echoToolDef := llm.ToolDefinition{
// 		Name:        "echo",
// 		Description: "Echoes back the input",
// 		Parameters: llm.Schema{
// 			Type:     "object",
// 			Required: []string{"text"},
// 			Properties: map[string]*llm.SchemaProperty{
// 				"text": {Type: "string", Description: "The text to echo back"},
// 			},
// 		},
// 	}

// 	var echoInput string

// 	echoFunc := func(ctx context.Context, input string) (string, error) {
// 		var m map[string]interface{}
// 		err := json.Unmarshal([]byte(input), &m)
// 		require.NoError(t, err)
// 		echoInput = m["text"].(string)
// 		return input, nil
// 	}

// 	agent, err := New(Options{
// 		Model: anthropic.New(),
// 		Tools: []llm.Tool{llm.NewTool(&echoToolDef, echoFunc)},
// 	})
// 	require.NoError(t, err)

// 	err = agent.Start(ctx)
// 	require.NoError(t, err)
// 	defer agent.Stop(ctx)

// 	stream, err := agent.Chat(ctx, llm.Messages{llm.NewUserMessage("Please use the echo tool to echo 'hello world'")})
// 	require.NoError(t, err)

// 	messages, err := dive.ReadMessages(ctx, stream)
// 	require.NoError(t, err)
// 	require.Greater(t, len(messages), 0)

// 	lastMessage := messages[len(messages)-1]
// 	text := strings.ToLower(lastMessage.Text())
// 	require.Contains(t, text, "echo")
// 	require.Equal(t, "hello world", echoInput)

// 	err = agent.Stop(ctx)
// 	require.NoError(t, err)
// }

func TestAgentChatSystemPrompt(t *testing.T) {
	agent, err := New(Options{
		Name:         "TestAgent",
		Goal:         "Help research a topic.",
		Backstory:    "You are a research assistant.",
		IsSupervisor: false,
		Model:        anthropic.New(),
	})
	require.NoError(t, err)

	// Get the chat system prompt
	chatSystemPrompt, err := agent.buildSystemPrompt("chat")
	require.NoError(t, err)

	// Verify that the chat system prompt doesn't contain the status section
	require.NotContains(t, chatSystemPrompt, "<status>")
	require.NotContains(t, chatSystemPrompt, "active")
	require.NotContains(t, chatSystemPrompt, "completed")
	require.NotContains(t, chatSystemPrompt, "paused")
	require.NotContains(t, chatSystemPrompt, "blocked")
	require.NotContains(t, chatSystemPrompt, "error")

	// Get the task system prompt
	taskSystemPrompt, err := agent.buildSystemPrompt("task")
	require.NoError(t, err)

	// Verify that the task system prompt contains the status section
	require.Contains(t, taskSystemPrompt, "<status>")
	require.Contains(t, taskSystemPrompt, "active")
	require.Contains(t, taskSystemPrompt, "completed")
	require.Contains(t, taskSystemPrompt, "paused")
	require.Contains(t, taskSystemPrompt, "blocked")
	require.Contains(t, taskSystemPrompt, "error")
}

// TestAgentCreateResponse demonstrates using the CreateResponse API
func TestAgentCreateResponse(t *testing.T) {
	// Setup a simple mock LLM
	mockLLM := &mockLLM{
		generateFunc: func(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (*llm.Response, error) {
			return &llm.Response{
				ID:    "resp_123",
				Model: "test-model",
				Role:  llm.Assistant,
				Content: []*llm.Content{
					{Type: llm.ContentTypeText, Text: "This is a test response"},
				},
				Type:       "message",
				StopReason: "stop",
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
			}, nil
		},
	}

	// Create a simple agent with the mock LLM
	agent, err := New(Options{
		Name:  "TestAgent",
		Goal:  "To test the CreateResponse API",
		Model: mockLLM,
	})
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	t.Run("CreateResponse with input string", func(t *testing.T) {
		// Test with a simple string input
		resp, err := agent.CreateResponse(context.Background(), dive.WithInput("Hello, agent!"))
		if err != nil {
			t.Fatalf("CreateResponse failed: %v", err)
		}

		if resp.Text != "This is a test response" {
			t.Errorf("Expected 'This is a test response', got %q", resp.Text)
		}

		if resp.Model != "test-model" {
			t.Errorf("Expected model 'test-model', got %q", resp.Model)
		}

		if resp.TokenUsage == nil {
			t.Errorf("Expected non-nil TokenUsage")
		} else {
			if resp.TokenUsage.InputTokens != 10 {
				t.Errorf("Expected InputTokens=10, got %d", resp.TokenUsage.InputTokens)
			}
			if resp.TokenUsage.OutputTokens != 5 {
				t.Errorf("Expected OutputTokens=5, got %d", resp.TokenUsage.OutputTokens)
			}
		}
	})

	t.Run("CreateResponse with messages", func(t *testing.T) {
		// Test with explicit messages
		messages := []*llm.Message{
			llm.NewUserMessage("Here's a more complex message"),
		}

		resp, err := agent.CreateResponse(context.Background(), dive.WithMessages(messages))
		if err != nil {
			t.Fatalf("CreateResponse with messages failed: %v", err)
		}

		if resp.Text != "This is a test response" {
			t.Errorf("Expected 'This is a test response', got %q", resp.Text)
		}
	})
}

// // TestAgentStreamResponse demonstrates using the StreamResponse API
// func TestAgentStreamResponse(t *testing.T) {
// 	// Setup a mock streaming LLM
// 	mockStreamingLLM := &mockStreamingLLM{
// 		generateFunc: func(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (*llm.Response, error) {
// 			return &llm.Response{
// 				ID:    "resp_123",
// 				Model: "test-model",
// 				Role:  llm.Assistant,
// 				Content: []*llm.Content{
// 					{Type: llm.ContentTypeText, Text: "This is a test response"},
// 				},
// 				Type:       "message",
// 				StopReason: "stop",
// 				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
// 			}, nil
// 		},
// 		streamFunc: func(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (llm.StreamIterator, error) {
// 			return &mockStreamIterator{
// 				events: []*llm.Event{
// 					{
// 						Type: llm.EventTypeMessageStart,
// 						Message: &llm.Response{
// 							ID:    "resp_123",
// 							Model: "test-model",
// 							Role:  llm.Assistant,
// 							Type:  "message",
// 						},
// 					},
// 					{
// 						Type: llm.EventTypeContentBlockStart,
// 						Index: func() *int {
// 							i := 0
// 							return &i
// 						}(),
// 						ContentBlock: &llm.EventContentBlock{
// 							Type: llm.ContentTypeText,
// 						},
// 					},
// 					{
// 						Type: llm.EventTypeContentBlockDelta,
// 						Index: func() *int {
// 							i := 0
// 							return &i
// 						}(),
// 						Delta: &llm.EventDelta{
// 							Type: llm.EventDeltaTypeText,
// 							Text: "This is a ",
// 						},
// 					},
// 					{
// 						Type: llm.EventTypeContentBlockDelta,
// 						Index: func() *int {
// 							i := 0
// 							return &i
// 						}(),
// 						Delta: &llm.EventDelta{
// 							Type: llm.EventDeltaTypeText,
// 							Text: "test response",
// 						},
// 					},
// 					{
// 						Type: llm.EventTypeMessageStop,
// 						Usage: &llm.Usage{
// 							InputTokens:  10,
// 							OutputTokens: 5,
// 						},
// 					},
// 				},
// 			}, nil
// 		},
// 	}

// 	// Create a simple agent with the mock streaming LLM
// 	agent, err := New(Options{
// 		Name:  "TestStreamingAgent",
// 		Goal:  "To test the StreamResponse API",
// 		Model: mockStreamingLLM,
// 	})
// 	if err != nil {
// 		t.Fatalf("Failed to create streaming agent: %v", err)
// 	}

// 	t.Run("StreamResponse with input string", func(t *testing.T) {
// 		// Test with a simple string input
// 		stream, err := agent.StreamResponse(context.Background(), dive.WithInput("Hello, agent!"))
// 		if err != nil {
// 			t.Fatalf("StreamResponse failed: %v", err)
// 		}

// 		// Consume the stream to verify it works
// 		var events []*dive.Event
// 		for stream.Next(context.Background()) {
// 			events = append(events, stream.Event())
// 		}

// 		if err := stream.Err(); err != nil {
// 			t.Fatalf("Stream error: %v", err)
// 		}

// 		if len(events) == 0 {
// 			t.Errorf("Expected events from stream, got none")
// 		}
// 	})
// }

// Mock types for testing

type mockLLM struct {
	generateFunc func(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (*llm.Response, error)
}

func (m *mockLLM) Name() string {
	return "mock-llm"
}

func (m *mockLLM) Generate(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (*llm.Response, error) {
	return m.generateFunc(ctx, messages, opts...)
}

type mockStreamingLLM struct {
	mockLLM
	streamFunc func(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (llm.StreamIterator, error)
}

func (m *mockStreamingLLM) Stream(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (llm.StreamIterator, error) {
	return m.streamFunc(ctx, messages, opts...)
}

type mockStreamIterator struct {
	events  []*llm.Event
	current int
	err     error
}

func (m *mockStreamIterator) Next() bool {
	if m.current >= len(m.events) {
		return false
	}
	m.current++
	return true
}

func (m *mockStreamIterator) Event() *llm.Event {
	if m.current == 0 || m.current > len(m.events) {
		return nil
	}
	return m.events[m.current-1]
}

func (m *mockStreamIterator) Err() error {
	return m.err
}

func (m *mockStreamIterator) Close() error {
	return nil
}
