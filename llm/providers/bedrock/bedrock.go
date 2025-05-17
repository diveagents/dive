package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"bytes"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/diveagents/dive/llm"
)

// var _ llm.StreamingLLM = &Provider{}

const (
	ModelTypeClaude = "anthropic"
	ModelTypeTitan  = "amazon"
)

type Provider struct {
	client *bedrockruntime.Client
	model  string
}

type Option func(*Provider)

func WithClient(client *bedrockruntime.Client) Option {
	return func(p *Provider) {
		p.client = client
	}
}

func WithModel(model string) Option {
	return func(p *Provider) {
		p.model = model
	}
}

func New(opts ...Option) *Provider {
	p := &Provider{
		model: "anthropic.claude-v2",
	}
	for _, opt := range opts {
		opt(p)
	}

	// Load AWS configuration
	sdkConfig, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
	if err != nil {
		fmt.Println("Couldn't load default configuration. Have you set up your AWS account?")
		fmt.Println(err)
		panic(err)
	}

	client := bedrockruntime.NewFromConfig(sdkConfig)
	p.client = client
	return p
}

func (p *Provider) Name() string {
	return fmt.Sprintf("bedrock-%s", p.model)
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
		model = p.model
	}

	maxTokens := config.MaxTokens
	if maxTokens == nil {
		maxTokens = new(int)
		*maxTokens = 4096 // Default max tokens
	}

	messageCount := len(messages)
	if messageCount == 0 {
		return nil, fmt.Errorf("no messages provided")
	}
	for i, message := range messages {
		if len(message.Content) == 0 {
			return nil, fmt.Errorf("empty message detected (index %d)", i)
		}
	}

	// Convert messages to Anthropic format
	prompt, err := convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("error converting messages: %w", err)
	}

	reqBody := Request{
		Prompt:            prompt,
		MaxTokensToSample: *maxTokens,
		Temperature:       0.7,
	}

	if config.Temperature != nil {
		reqBody.Temperature = *config.Temperature
	}

	modelType := getModelType(model)
	reqBody, err = buildRequest(messages, *maxTokens, reqBody.Temperature, false, modelType)
	if err != nil {
		return nil, fmt.Errorf("error building request: %w", err)
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	output, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(model),
		ContentType: aws.String("application/json"),
		Body:        jsonBody,
	})
	if err != nil {
		return nil, fmt.Errorf("error invoking model: %w", err)
	}

	var result Response
	if err := json.Unmarshal(output.Body, &result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	fmt.Printf("result: %+v\n", result)

	response := llm.NewResponse(llm.ResponseOptions{
		Model: model,
		Role:  llm.Assistant,
		Message: llm.NewMessage(llm.Assistant, []*llm.Content{
			{Type: llm.ContentTypeText, Text: result.Completion},
		}),
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

// func (p *Provider) Stream(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (llm.StreamIterator, error) {
// 	config := &llm.Config{}
// 	for _, opt := range opts {
// 		opt(config)
// 	}

// 	model := config.Model
// 	if model == "" {
// 		model = p.model
// 	}

// 	maxTokens := config.MaxTokens
// 	if maxTokens == nil {
// 		maxTokens = new(int)
// 		*maxTokens = 4096 // Default max tokens
// 	}

// 	messageCount := len(messages)
// 	if messageCount == 0 {
// 		return nil, fmt.Errorf("no messages provided")
// 	}
// 	for i, message := range messages {
// 		if len(message.Content) == 0 {
// 			return nil, fmt.Errorf("empty message detected (index %d)", i)
// 		}
// 	}

// 	// Convert messages to Anthropic format
// 	prompt, err := convertMessages(messages)
// 	if err != nil {
// 		return nil, fmt.Errorf("error converting messages: %w", err)
// 	}

// 	reqBody := Request{
// 		Prompt:            prompt,
// 		MaxTokensToSample: *maxTokens,
// 		Temperature:       0.7,
// 		Stream:            true,
// 	}

// 	if config.Temperature != nil {
// 		reqBody.Temperature = *config.Temperature
// 	}

// 	modelType := getModelType(model)
// 	reqBody, err = buildRequest(messages, *maxTokens, reqBody.Temperature, true, modelType)
// 	if err != nil {
// 		return nil, fmt.Errorf("error building request: %w", err)
// 	}

// 	jsonBody, err := json.Marshal(reqBody)
// 	if err != nil {
// 		return nil, fmt.Errorf("error marshaling request: %w", err)
// 	}

// 	output, err := p.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
// 		ModelId:     aws.String(model),
// 		ContentType: aws.String("application/json"),
// 		Body:        jsonBody,
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("error invoking model: %w", err)
// 	}

// 	return NewStreamIterator(output), nil
// }

func (p *Provider) SupportsStreaming() bool {
	return false
}

// Helper types and functions
type Request struct {
	Prompt            string    `json:"prompt,omitempty"`
	MaxTokensToSample int       `json:"max_tokens_to_sample,omitempty"`
	Messages          []Message `json:"messages,omitempty"`
	MaxTokens         int       `json:"max_tokens,omitempty"`
	Temperature       float64   `json:"temperature,omitempty"`
	StopSequences     []string  `json:"stop_sequences,omitempty"`
	Stream            bool      `json:"stream,omitempty"`
}

type Response struct {
	Completion string `json:"completion"`
}

type StreamResponse struct {
	Completion string `json:"completion"`
	Stop       string `json:"stop,omitempty"`
}

// StreamChunk struct
type StreamChunk struct {
	Type       string `json:"type"`
	Completion string `json:"completion"`
	Stop       string `json:"stop"`
}

// StreamIterator implements the llm.StreamIterator interface
type StreamIterator struct {
	stream       *bedrockruntime.InvokeModelWithResponseStreamEventStream
	events       <-chan types.ResponseStream
	err          error
	currentChunk *StreamChunk
}

func NewStreamIterator(output *bedrockruntime.InvokeModelWithResponseStreamOutput) *StreamIterator {
	stream := output.GetStream()
	return &StreamIterator{
		stream: stream,
		events: stream.Events(),
	}
}

func (s *StreamIterator) Next() bool {
	event, ok := <-s.events
	if !ok {
		return false
	}

	if err := s.stream.Err(); err != nil {
		s.err = err
		return false
	}

	switch v := event.(type) {
	case *types.ResponseStreamMemberChunk:
		var chunk StreamChunk
		if err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(&chunk); err != nil {
			s.err = fmt.Errorf("failed to unmarshal chunk: %w", err)
			return false
		}

		if chunk.Type == "stop" {
			return false
		}

		s.currentChunk = &chunk
		return true

	case *types.UnknownUnionMember:
		s.err = fmt.Errorf("unknown event type with tag: %s", v.Tag)
		return false

	default:
		s.err = fmt.Errorf("unexpected event type: %T", event)
		return false
	}
}

func (s *StreamIterator) Event() *llm.Event {
	return &llm.Event{
		Type: llm.EventContentBlockDelta,
		Delta: &llm.Delta{
			Type: "text_delta",
			Text: s.currentChunk.Completion,
		},
	}
}

func (s *StreamIterator) Close() error {
	return s.stream.Close()
}

func (s *StreamIterator) Err() error {
	return s.err
}

// Helper function to convert messages to Anthropic format
func convertMessages(messages []*llm.Message) (string, error) {
	var prompt string
	for _, msg := range messages {
		switch msg.Role {
		case llm.User:
			prompt += "\n\nHuman: " + msg.CompleteText()
		case llm.Assistant:
			prompt += "\n\nAssistant: " + msg.CompleteText()
		case llm.System:
			// System messages are not supported in the same way by Claude on Bedrock
			// We'll prepend it to the first user message
			continue
		}
	}
	prompt += "\n\nAssistant:"
	return prompt, nil
}

func getModelType(model string) string {
	if strings.HasPrefix(model, "anthropic.") {
		return ModelTypeClaude
	}
	if strings.HasPrefix(model, "amazon.") {
		return ModelTypeTitan
	}
	return ModelTypeClaude // default to Claude format
}

// Add a new struct for the content
type MessageContent struct {
	Text string `json:"text"`
}

type Message struct {
	Role    string           `json:"role"`
	Content []MessageContent `json:"content"` // Changed to []MessageContent
}

func buildRequest(messages []*llm.Message, maxTokens int, temperature float64, stream bool, modelType string) (Request, error) {
	req := Request{}

	switch modelType {
	case ModelTypeTitan:
		var titanMessages []Message
		for _, msg := range messages {
			role := "user"
			if msg.Role == llm.Assistant {
				role = "assistant"
			} else if msg.Role == llm.System {
				role = "system"
			}
			titanMessages = append(titanMessages, Message{
				Role: role,
				Content: []MessageContent{
					{Text: msg.CompleteText()},
				},
			})
		}
		req.Messages = titanMessages

	default: // Claude format
		req.MaxTokensToSample = maxTokens
		req.Stream = stream
		req.Temperature = temperature
		prompt, err := convertMessages(messages)
		if err != nil {
			return Request{}, err
		}
		req.Prompt = prompt
	}

	return req, nil
}
