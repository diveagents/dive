package groq

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/getstingrai/agents/llm"
	"github.com/getstingrai/agents/providers"
	"github.com/getstingrai/agents/retry"
)

var (
	DefaultModel            = "llama-3.3-70b-versatile"
	DefaultMessagesEndpoint = "https://api.groq.com/openai/v1/chat/completions"
	DefaultMaxTokens        = 4096
)

var _ llm.LLM = &Provider{}

type Provider struct {
	apiKey           string
	messagesEndpoint string
	maxTokens        int
	client           *http.Client
}

func New() *Provider {
	return &Provider{
		apiKey:           os.Getenv("GROQ_API_KEY"),
		messagesEndpoint: DefaultMessagesEndpoint,
		maxTokens:        DefaultMaxTokens,
		client:           http.DefaultClient,
	}
}

func (p *Provider) WithMaxTokens(maxTokens int) *Provider {
	p.maxTokens = maxTokens
	return p
}

func (p *Provider) WithMessagesEndpoint(messagesEndpoint string) *Provider {
	p.messagesEndpoint = messagesEndpoint
	return p
}

func (p *Provider) WithClient(client *http.Client) *Provider {
	p.client = client
	return p
}

func (p *Provider) WithAPIKey(apiKey string) *Provider {
	p.apiKey = apiKey
	return p
}

func (p *Provider) Generate(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (*llm.Response, error) {
	config := &llm.Config{}
	for _, opt := range opts {
		opt(config)
	}

	if hooks := config.Hooks[llm.BeforeGenerate]; hooks != nil {
		hooks(ctx, &llm.HookContext{
			Type:     llm.BeforeGenerate,
			Messages: messages,
		})
	}

	model := config.Model
	if model == "" {
		model = DefaultModel
	}

	maxTokens := config.MaxTokens
	if maxTokens == nil {
		maxTokens = &p.maxTokens
	}

	msgs, err := convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("error converting messages: %w", err)
	}

	var tools []Tool
	for _, tool := range config.Tools {
		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        tool.Definition().Name,
				Description: tool.Definition().Description,
				Parameters:  tool.Definition().Parameters,
			},
		})
	}

	var toolChoice string
	if config.ToolChoice.Type != "" {
		toolChoice = config.ToolChoice.Type
	} else if len(tools) > 0 {
		toolChoice = "auto"
	}

	reqBody := Request{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: config.Temperature,
		Tools:       tools,
		ToolChoice:  toolChoice,
	}

	if config.SystemPrompt != "" {
		reqBody.Messages = append([]Message{{
			Role:    "system",
			Content: config.SystemPrompt,
		}}, reqBody.Messages...)
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	var result Response
	err = retry.Do(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, "POST", p.messagesEndpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("error creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("Content-Type", "application/json")
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("error making request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return providers.NewError(resp.StatusCode, string(body))
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("error decoding response: %w", err)
		}
		return nil
	}, retry.WithMaxRetries(5))
	if err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("empty response from groq api")
	}
	choice := result.Choices[0]

	var toolCalls []llm.ToolCall
	for _, toolCall := range choice.Message.ToolCalls {
		toolCalls = append(toolCalls, llm.ToolCall{
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: toolCall.Function.Arguments,
		})
	}

	response := llm.NewResponse(llm.ResponseOptions{
		ID:    result.ID,
		Model: model,
		Role:  llm.Assistant,
		Usage: llm.Usage{
			InputTokens:  result.Usage.PromptTokens,
			OutputTokens: result.Usage.CompletionTokens,
		},
		Message: &llm.Message{
			Role: llm.Assistant,
			Content: []*llm.Content{{
				Type: llm.ContentTypeText,
				Text: choice.Message.Content,
			}},
		},
		ToolCalls: toolCalls,
	})

	if hooks := config.Hooks[llm.AfterGenerate]; hooks != nil {
		hooks(ctx, &llm.HookContext{
			Type:     llm.AfterGenerate,
			Messages: messages,
			Response: response,
		})
	}

	return response, nil
}

func (p *Provider) Stream(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (llm.Stream, error) {
	config := &llm.Config{}
	for _, opt := range opts {
		opt(config)
	}

	model := config.Model
	if model == "" {
		model = DefaultModel
	}

	msgs, err := convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("error converting messages: %w", err)
	}

	maxTokens := config.MaxTokens
	if maxTokens == nil {
		maxTokens = &p.maxTokens
	}

	reqBody := Request{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: config.Temperature,
		Stream:      true,
	}

	if config.SystemPrompt != "" {
		reqBody.Messages = append([]Message{{
			Role:    "system",
			Content: config.SystemPrompt,
		}}, reqBody.Messages...)
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.messagesEndpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("error from API (status %d): %s", resp.StatusCode, string(body))
	}

	return &Stream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, nil
}

func (p *Provider) SupportsStreaming() bool {
	return true
}

func convertMessages(messages []*llm.Message) ([]Message, error) {
	var result []Message

	for _, msg := range messages {
		content := ""
		for _, c := range msg.Content {
			switch c.Type {
			case llm.ContentTypeText:
				content += c.Text
			case llm.ContentTypeToolResult:
				result = append(result, Message{
					Role:       "tool",
					Content:    c.Text,
					ToolCallID: c.ToolUseID,
					Name:       c.Name,
				})
				fmt.Println("tool result", "TCID:", c.ToolUseID, "Name:", c.Name)
			default:
				return nil, fmt.Errorf("unsupported content type: %s", c.Type)
			}
		}

		result = append(result, Message{
			Role:    strings.ToLower(string(msg.Role)),
			Content: content,
		})
	}

	return result, nil
}

type Stream struct {
	reader *bufio.Reader
	body   io.ReadCloser
	err    error
}

func (s *Stream) Next(ctx context.Context) (*llm.StreamEvent, bool) {
	for {
		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			s.err = err
			return nil, false
		}

		// Skip empty lines
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		// Remove "data: " prefix if present
		line = bytes.TrimPrefix(line, []byte("data: "))

		// Check for stream end
		if bytes.Equal(bytes.TrimSpace(line), []byte("[DONE]")) {
			return nil, false
		}

		var event StreamResponse
		if err := json.Unmarshal(line, &event); err != nil {
			continue // Skip malformed events
		}

		if len(event.Choices) > 0 {
			choice := event.Choices[0]
			if choice.Delta.Content != "" {
				return &llm.StreamEvent{
					Type: llm.EventContentBlockDelta,
					Delta: &llm.Delta{
						Type: "text_delta",
						Text: choice.Delta.Content,
					},
				}, true
			}
		}
	}
}

func (s *Stream) Close() error {
	return s.body.Close()
}

func (s *Stream) Err() error {
	return s.err
}
