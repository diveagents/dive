package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/getstingrai/dive/llm"
	"github.com/stretchr/testify/require"
)

func TestHelloWorld(t *testing.T) {
	ctx := context.Background()
	provider := New()
	response, err := provider.Generate(ctx, []*llm.Message{
		llm.NewUserMessage("respond with \"hello\""),
	})
	require.NoError(t, err)
	require.Equal(t, "hello", response.Message().Text())
}

func TestHelloWorldStream(t *testing.T) {
	ctx := context.Background()
	provider := New()
	stream, err := provider.Stream(ctx, []*llm.Message{
		llm.NewUserMessage("count to 10. respond with the integers only, separated by spaces."),
	})
	require.NoError(t, err)

	var events []*llm.StreamEvent
	for {
		event, ok := stream.Next(ctx)
		if !ok {
			break
		}
		events = append(events, event)
	}

	var finalText string
	var texts []string
	for _, event := range events {
		switch event.Type {
		case llm.EventContentBlockDelta:
			numbers := strings.FieldsFunc(event.Delta.Text, func(r rune) bool {
				return r == '\n' || r == ' '
			})
			texts = append(texts, numbers...)
			finalText = event.AccumulatedText
		}
	}

	expectedOutput := "1 2 3 4 5 6 7 8 9 10"
	normalizedFinalText := strings.Join(strings.Fields(finalText), " ")
	normalizedTexts := strings.Join(texts, " ")

	require.Equal(t, expectedOutput, normalizedFinalText)
	require.Equal(t, expectedOutput, normalizedTexts)
}

func addFunc(ctx context.Context, input string) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", params["a"].(int)+params["b"].(int)), nil
}

func TestToolUse(t *testing.T) {
	ctx := context.Background()
	provider := New()

	messages := []*llm.Message{
		llm.NewUserMessage("add 567 and 111"),
	}

	add := llm.ToolDefinition{
		Name:        "add",
		Description: "Returns the sum of two numbers, \"a\" and \"b\"",
		Parameters: llm.Schema{
			Type:     "object",
			Required: []string{"a", "b"},
			Properties: map[string]*llm.SchemaProperty{
				"a": {Type: "number", Description: "The first number"},
				"b": {Type: "number", Description: "The second number"},
			},
		},
	}

	response, err := provider.Generate(ctx, messages,
		llm.WithTools(llm.NewTool(&add, addFunc)),
		llm.WithToolChoice(llm.ToolChoice{
			Type: "tool",
			Name: "add",
		}),
	)
	require.NoError(t, err)

	require.Equal(t, 1, len(response.Message().Content))
	content := response.Message().Content[0]
	require.Equal(t, llm.ContentTypeToolUse, content.Type)
	require.Equal(t, "add", content.Name)
	require.Equal(t, `{"a":567,"b":111}`, string(content.Input))
}
