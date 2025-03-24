package vertex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

const (
	DefaultVersion = "vertex-2023-10-16"
)

var _ llm.StreamingLLM = &Provider{}

type Provider struct {
	client    *http.Client
	endpoint  string
	model     string
	region    string
	projectID string
	version   string
}

type Option func(*Provider)

func WithClient(client *http.Client) Option {
	return func(p *Provider) {
		p.client = client
	}
}

func WithEndpoint(endpoint string) Option {
	return func(p *Provider) {
		p.endpoint = endpoint
	}
}

func WithModel(model string) Option {
	return func(p *Provider) {
		p.model = model
	}
}

func WithRegion(region string) Option {
	return func(p *Provider) {
		p.region = region
	}
}

func WithProjectID(projectID string) Option {
	return func(p *Provider) {
		p.projectID = projectID
	}
}

func WithVersion(version string) Option {
	return func(p *Provider) {
		p.version = version
	}
}

func New(opts ...Option) *Provider {
	p := &Provider{
		client:  http.DefaultClient,
		version: DefaultVersion,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func WithGoogleAuth(ctx context.Context, region string, projectID string, scopes ...string) (Option, error) {
	if region == "" {
		return nil, fmt.Errorf("region must be provided")
	}
	creds, err := google.FindDefaultCredentials(ctx, scopes...)
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}
	return WithCredentials(ctx, region, projectID, creds)
}

func WithCredentials(ctx context.Context, region string, projectID string, creds *google.Credentials) (Option, error) {
	client, _, err := transport.NewHTTPClient(ctx, option.WithTokenSource(creds.TokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return func(p *Provider) {
		p.client = client
		p.region = region
		p.projectID = projectID
		p.endpoint = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1", region)
	}, nil
}

func (p *Provider) Name() string {
	return fmt.Sprintf("vertex-%s", p.model)
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
	msgs, err := convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("error converting messages: %w", err)
	}

	reqBody := Request{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: config.Temperature,
		System:      config.SystemPrompt,
	}

	if len(config.Tools) > 0 {
		var tools []*Tool
		for _, tool := range config.Tools {
			toolDef := tool.Definition()
			tools = append(tools, &Tool{
				Name:        toolDef.Name,
				Description: toolDef.Description,
				InputSchema: toolDef.Parameters,
			})
		}
		reqBody.Tools = tools
	}
	if config.ToolChoice.Type != "" {
		reqBody.ToolChoice = &config.ToolChoice
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Add Vertex AI specific headers and path
	if !gjson.GetBytes(jsonBody, "anthropic_version").Exists() {
		jsonBody, _ = sjson.SetBytes(jsonBody, "anthropic_version", p.version)
	}

	if p.projectID == "" {
		return nil, fmt.Errorf("no projectId was given and it could not be resolved from credentials")
	}

	url := fmt.Sprintf("%s/projects/%s/locations/%s/publishers/anthropic/models/%s:rawPredict",
		p.endpoint, p.projectID, p.region, model)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, providers.NewError(resp.StatusCode, string(body))
	}

	var result Response
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty response from vertex api")
	}

	toolCalls, contentBlocks := processContentBlocks(result.Content)

	response := llm.NewResponse(llm.ResponseOptions{
		ID:         result.ID,
		Model:      model,
		Role:       llm.Assistant,
		StopReason: result.StopReason,
		Usage: llm.Usage{
			InputTokens:  result.Usage.InputTokens,
			OutputTokens: result.Usage.OutputTokens,
		},
		ToolCalls: toolCalls,
		Message:   llm.NewMessage(llm.Assistant, contentBlocks),
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

func (p *Provider) Stream(ctx context.Context, messages []*llm.Message, opts ...llm.Option) (llm.StreamIterator, error) {
	config := &llm.Config{}
	for _, opt := range opts {
		opt(config)
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

	msgs, err := convertMessages(messages)
	if err != nil {
		return nil, fmt.Errorf("error converting messages: %w", err)
	}

	reqBody := Request{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   maxTokens,
		Temperature: config.Temperature,
		System:      config.SystemPrompt,
		Stream:      true,
	}

	if len(config.Tools) > 0 {
		var tools []*Tool
		for _, tool := range config.Tools {
			toolDef := tool.Definition()
			tools = append(tools, &Tool{
				Name:        toolDef.Name,
				Description: toolDef.Description,
				InputSchema: toolDef.Parameters,
			})
		}
		reqBody.Tools = tools
	}
	if config.ToolChoice.Type != "" {
		reqBody.ToolChoice = &config.ToolChoice
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	// Add Vertex AI specific headers and path
	if !gjson.GetBytes(jsonBody, "anthropic_version").Exists() {
		jsonBody, _ = sjson.SetBytes(jsonBody, "anthropic_version", p.version)
	}

	if p.projectID == "" {
		return nil, fmt.Errorf("no projectId was given and it could not be resolved from credentials")
	}

	url := fmt.Sprintf("%s/projects/%s/locations/%s/publishers/anthropic/models/%s:streamRawPredict",
		p.endpoint, p.projectID, p.region, model)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, providers.NewError(resp.StatusCode, string(body))
	}

	return &StreamIterator{
		reader:        bufio.NewReader(resp.Body),
		body:          resp.Body,
		contentBlocks: make(map[int]*ContentBlockAccumulator),
		responseID:    "",
		responseModel: "",
		usage:         Usage{},
		closeOnce:     sync.Once{},
	}, nil
}

func (p *Provider) SupportsStreaming() bool {
	return true
}

// Helper types and functions from the anthropic provider
type Message struct {
	ID      string          `json:"id"`
	Model   string          `json:"model"`
	Role    string          `json:"role"`
	Content []*ContentBlock `json:"content"`
	Usage   Usage           `json:"usage"`
}

type ContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	Source    *ImageSource    `json:"source,omitempty"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema llm.Schema `json:"parameters"`
}

type Request struct {
	Model       string          `json:"model"`
	Messages    []*Message      `json:"messages"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	System      string          `json:"system,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []*Tool         `json:"tools,omitempty"`
	ToolChoice  *llm.ToolChoice `json:"tool_choice,omitempty"`
}

type Response struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Content    []*ContentBlock `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Usage      Usage           `json:"usage"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamIterator implements the llm.StreamIterator interface
type StreamIterator struct {
	reader        *bufio.Reader
	body          io.ReadCloser
	err           error
	currentEvent  *llm.Event
	contentBlocks map[int]*ContentBlockAccumulator
	responseID    string
	responseModel string
	usage         Usage
	closeOnce     sync.Once
}

type ContentBlockAccumulator struct {
	Type        string
	Text        string
	PartialJSON string
	ToolUse     *ToolUse
}

type ToolUse struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// Helper functions
func convertMessages(messages []*llm.Message) ([]*Message, error) {
	var result []*Message
	for _, msg := range messages {
		var blocks []*ContentBlock
		for _, c := range msg.Content {
			switch c.Type {
			case llm.ContentTypeText:
				blocks = append(blocks, &ContentBlock{
					Type: "text",
					Text: c.Text,
				})
			case llm.ContentTypeImage:
				blocks = append(blocks, &ContentBlock{
					Type: "image",
					Source: &ImageSource{
						Type:      "base64",
						MediaType: c.MediaType,
						Data:      c.Data,
					},
				})
			case llm.ContentTypeToolUse:
				blocks = append(blocks, &ContentBlock{
					Type:  "tool_use",
					ID:    c.ID,
					Name:  c.Name,
					Input: json.RawMessage(c.Input),
				})
			case llm.ContentTypeToolResult:
				blocks = append(blocks, &ContentBlock{
					Type:      "tool_result",
					ToolUseID: c.ToolUseID,
					Content:   c.Text, // oddly we have to rename to "content"
				})
			default:
				return nil, fmt.Errorf("unsupported content type: %s", c.Type)
			}
		}
		result = append(result, &Message{
			ID:      msg.ID,
			Role:    strings.ToLower(string(msg.Role)),
			Content: blocks,
		})
	}
	return result, nil
}

func processContentBlocks(blocks []*ContentBlock) ([]llm.ToolCall, []*llm.Content) {
	var toolCalls []llm.ToolCall
	var contentBlocks []*llm.Content

	for _, block := range blocks {
		switch block.Type {
		case "text":
			contentBlocks = append(contentBlocks, &llm.Content{
				Type: llm.ContentTypeText,
				Text: block.Text,
			})
		case "tool_use":
			toolCalls = append(toolCalls, llm.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: string(block.Input),
			})
			contentBlocks = append(contentBlocks, &llm.Content{
				Type:  llm.ContentTypeToolUse,
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return toolCalls, contentBlocks
}

// StreamIterator methods
func (s *StreamIterator) Next() bool {
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		s.err = err
		return false
	}
	// Skip empty lines
	if len(bytes.TrimSpace(line)) == 0 {
		return true
	}
	// Parse the event type from the SSE format
	if bytes.HasPrefix(line, []byte("event: ")) {
		return true
	}
	// Remove "data: " prefix if present
	line = bytes.TrimPrefix(line, []byte("data: "))

	// Check for stream end
	if bytes.Equal(bytes.TrimSpace(line), []byte("[DONE]")) {
		return false
	}

	var event StreamEvent
	if err := json.Unmarshal(line, &event); err != nil {
		s.err = err
		return false
	}

	switch llm.EventType(event.Type) {
	case llm.EventMessageStart:
		s.responseID = event.Message.ID
		s.responseModel = event.Message.Model
		s.usage = event.Message.Usage
		s.currentEvent = &llm.Event{
			Type:  llm.EventMessageStart,
			Index: event.Index,
			Message: &llm.Message{
				ID:      event.Message.ID,
				Role:    llm.Assistant,
				Content: []*llm.Content{},
			},
			Usage: &llm.Usage{
				InputTokens:  event.Message.Usage.InputTokens,
				OutputTokens: event.Message.Usage.OutputTokens,
			},
		}

	case llm.EventContentBlockStart:
		s.contentBlocks[event.Index] = &ContentBlockAccumulator{
			Type: event.ContentBlock.Type,
			Text: event.ContentBlock.Text,
		}
		if event.ContentBlock.Type == "tool_use" {
			s.contentBlocks[event.Index].ToolUse = &ToolUse{
				ID:   event.ContentBlock.ID,
				Name: event.ContentBlock.Name,
			}
		}
		s.currentEvent = &llm.Event{
			Type:  llm.EventContentBlockStart,
			Index: event.Index,
			ContentBlock: &llm.ContentBlock{
				ID:   event.ContentBlock.ID,
				Name: event.ContentBlock.Name,
				Type: event.ContentBlock.Type,
				Text: event.ContentBlock.Text,
			},
		}

	case llm.EventContentBlockDelta:
		block := s.contentBlocks[event.Index]
		if block == nil {
			block = &ContentBlockAccumulator{Type: event.Delta.Type}
			s.contentBlocks[event.Index] = block
		}
		block.Text += event.Delta.Text
		if event.Delta.Type == "input_json_delta" {
			block.PartialJSON += event.Delta.PartialJSON
		}
		if block.Type == "tool_use" && block.ToolUse == nil {
			block.ToolUse = &ToolUse{}
		}
		s.currentEvent = &llm.Event{
			Type:  llm.EventContentBlockDelta,
			Index: event.Index,
			Delta: &llm.Delta{
				Type:        event.Delta.Type,
				Text:        event.Delta.Text,
				PartialJSON: event.Delta.PartialJSON,
			},
		}

	case llm.EventMessageDelta:
		s.usage.InputTokens += event.Usage.InputTokens
		s.usage.OutputTokens += event.Usage.OutputTokens
		s.currentEvent = &llm.Event{
			Type:  llm.EventMessageDelta,
			Index: event.Index,
			Delta: &llm.Delta{
				Type:         "message_delta",
				StopReason:   event.Delta.StopReason,
				StopSequence: event.Delta.StopSequence,
			},
			Response: s.buildFinalResponse(event.Delta.StopReason),
		}
	}

	return true
}

func (s *StreamIterator) buildFinalResponse(stopReason string) *llm.Response {
	blocks := make([]*ContentBlock, 0, len(s.contentBlocks))
	for _, block := range s.contentBlocks {
		contentBlock := &ContentBlock{
			Type: block.Type,
			Text: block.Text,
		}
		if block.Type == "tool_use" && block.ToolUse != nil {
			contentBlock.ID = block.ToolUse.ID
			contentBlock.Name = block.ToolUse.Name
			contentBlock.Input = json.RawMessage(block.PartialJSON)
		}
		blocks = append(blocks, contentBlock)
	}
	toolCalls, contentBlocks := processContentBlocks(blocks)

	return llm.NewResponse(llm.ResponseOptions{
		ID:         s.responseID,
		Model:      s.responseModel,
		Role:       llm.Assistant,
		StopReason: stopReason,
		ToolCalls:  toolCalls,
		Message:    llm.NewMessage(llm.Assistant, contentBlocks),
		Usage: llm.Usage{
			InputTokens:  s.usage.InputTokens,
			OutputTokens: s.usage.OutputTokens,
		},
	})
}

func (s *StreamIterator) Event() *llm.Event {
	return s.currentEvent
}

func (s *StreamIterator) Close() error {
	var err error
	s.closeOnce.Do(func() { err = s.body.Close() })
	return err
}

func (s *StreamIterator) Err() error {
	return s.err
}

// Additional types needed for streaming
type StreamEvent struct {
	Type         string        `json:"type"`
	Index        int           `json:"index"`
	Message      *Message      `json:"message,omitempty"`
	ContentBlock *ContentBlock `json:"content_block,omitempty"`
	Delta        *Delta        `json:"delta,omitempty"`
	Usage        Usage         `json:"usage,omitempty"`
}

type Delta struct {
	Type         string  `json:"type,omitempty"`
	Text         string  `json:"text,omitempty"`
	StopReason   string  `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
	PartialJSON  string  `json:"partial_json,omitempty"`
}
