package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/llm/providers"
	"github.com/diveagents/dive/retry"
)

const ProviderName = "anthropic"

var (
	DefaultModel         = ModelClaudeSonnet4
	DefaultEndpoint      = "https://api.anthropic.com/v1/messages"
	DefaultMaxTokens     = 4096
	DefaultClient        = &http.Client{Timeout: 300 * time.Second}
	DefaultMaxRetries    = 6
	DefaultRetryBaseWait = 2 * time.Second
	DefaultVersion       = "2023-06-01"
)

var _ llm.StreamingLLM = &Provider{}

type Provider struct {
	client        *http.Client
	apiKey        string
	endpoint      string
	model         string
	maxTokens     int
	maxRetries    int
	retryBaseWait time.Duration
	version       string
}

func New(opts ...Option) *Provider {
	p := &Provider{
		apiKey:        os.Getenv("ANTHROPIC_API_KEY"),
		endpoint:      DefaultEndpoint,
		client:        DefaultClient,
		model:         DefaultModel,
		maxTokens:     DefaultMaxTokens,
		maxRetries:    DefaultMaxRetries,
		retryBaseWait: DefaultRetryBaseWait,
		version:       DefaultVersion,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Provider) Name() string {
	return ProviderName
}

func (p *Provider) Generate(ctx context.Context, opts ...llm.Option) (*llm.Response, error) {
	config := &llm.Config{}
	config.Apply(opts...)

	var request Request
	if err := p.applyRequestConfig(&request, config); err != nil {
		return nil, err
	}
	msgs, err := convertMessages(config.Messages)
	if err != nil {
		return nil, err
	}
	if config.Prefill != "" {
		msgs = append(msgs, llm.NewAssistantTextMessage(config.Prefill))
	}
	request.Messages = msgs

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	if err := config.FireHooks(ctx, &llm.HookContext{
		Type: llm.BeforeGenerate,
		Request: &llm.HookRequestContext{
			Messages: config.Messages,
			Config:   config,
			Body:     body,
		},
	}); err != nil {
		return nil, err
	}

	var result llm.Response
	err = retry.Do(ctx, func() error {
		req, err := p.createRequest(ctx, body, config, false)
		if err != nil {
			return err
		}
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("error making request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == 429 {
				if config.Logger != nil {
					config.Logger.Warn("rate limit exceeded",
						"status", resp.StatusCode, "body", string(body))
				}
			}
			return providers.NewError(resp.StatusCode, string(body))
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("error decoding response: %w", err)
		}
		return nil
	}, retry.WithMaxRetries(p.maxRetries), retry.WithBaseWait(p.retryBaseWait))

	if err != nil {
		return nil, err
	}
	if len(result.Content) == 0 {
		return nil, fmt.Errorf("empty response from anthropic api")
	}
	if config.Prefill != "" {
		addPrefill(result.Content, config.Prefill, config.PrefillClosingTag)
	}

	if err := config.FireHooks(ctx, &llm.HookContext{
		Type: llm.AfterGenerate,
		Request: &llm.HookRequestContext{
			Messages: config.Messages,
			Config:   config,
			Body:     body,
		},
		Response: &llm.HookResponseContext{
			Response: &result,
		},
	}); err != nil {
		return nil, err
	}
	return &result, nil
}

func (p *Provider) Stream(ctx context.Context, opts ...llm.Option) (llm.StreamIterator, error) {
	config := &llm.Config{}
	config.Apply(opts...)

	var request Request
	if err := p.applyRequestConfig(&request, config); err != nil {
		return nil, err
	}
	msgs, err := convertMessages(config.Messages)
	if err != nil {
		return nil, fmt.Errorf("error converting messages: %w", err)
	}
	if config.Prefill != "" {
		msgs = append(msgs, llm.NewAssistantTextMessage(config.Prefill))
	}
	request.Messages = msgs
	request.Stream = true

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %w", err)
	}

	if err := config.FireHooks(ctx, &llm.HookContext{
		Type: llm.BeforeGenerate,
		Request: &llm.HookRequestContext{
			Messages: config.Messages,
			Config:   config,
			Body:     body,
		},
	}); err != nil {
		return nil, err
	}

	var stream *StreamIterator
	err = retry.Do(ctx, func() error {
		req, err := p.createRequest(ctx, body, config, true)
		if err != nil {
			return err
		}
		resp, err := p.client.Do(req)
		if err != nil {
			return fmt.Errorf("error making request: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 429 {
				if config.Logger != nil {
					config.Logger.Warn("rate limit exceeded",
						"status", resp.StatusCode, "body", string(body))
				}
			}
			return providers.NewError(resp.StatusCode, string(body))
		}
		stream = &StreamIterator{
			body: resp.Body,
			reader: llm.NewServerSentEventsReader[llm.Event](resp.Body).
				WithSSECallback(config.SSECallback),
			prefill:           config.Prefill,
			prefillClosingTag: config.PrefillClosingTag,
		}
		return nil
	}, retry.WithMaxRetries(p.maxRetries), retry.WithBaseWait(p.retryBaseWait))

	if err != nil {
		return nil, err
	}
	return stream, nil
}

func convertMessages(messages []*llm.Message) ([]*llm.Message, error) {
	messageCount := len(messages)
	if messageCount == 0 {
		return nil, fmt.Errorf("no messages provided")
	}
	for i, message := range messages {
		if len(message.Content) == 0 {
			return nil, fmt.Errorf("empty message detected (index %d)", i)
		}
	}
	// Workaround for Anthropic bug
	reorderMessageContent(messages)
	// Anthropic errors if a message ID is set, so make a copy of the messages
	// and omit the ID field
	copied := make([]*llm.Message, len(messages))
	for i, message := range messages {
		// The "name" field in tool results can't be set either
		var copiedContent []llm.Content
		for _, content := range message.Content {
			switch c := content.(type) {
			case *llm.ToolResultContent:
				copiedContent = append(copiedContent, &llm.ToolResultContent{
					Content:      c.Content,
					ToolUseID:    c.ToolUseID,
					IsError:      c.IsError,
					CacheControl: c.CacheControl,
				})
			case *llm.DocumentContent:
				// Handle DocumentContent with file IDs for Anthropic API compatibility
				if c.Source != nil && c.Source.Type == llm.ContentSourceTypeFile && c.Source.FileID != "" {
					// For Anthropic API, file IDs are passed in the source structure
					docContent := &llm.DocumentContent{
						Title:        c.Title,
						Context:      c.Context,
						Citations:    c.Citations,
						CacheControl: c.CacheControl,
						Source: &llm.ContentSource{
							Type:   c.Source.Type,
							FileID: c.Source.FileID,
						},
					}
					copiedContent = append(copiedContent, docContent)
				} else {
					// Pass through other DocumentContent as-is
					copiedContent = append(copiedContent, content)
				}
			default:
				copiedContent = append(copiedContent, content)
			}
		}
		copied[i] = &llm.Message{
			Role:    message.Role,
			Content: copiedContent,
		}
	}
	return copied, nil
}

func (p *Provider) applyRequestConfig(req *Request, config *llm.Config) error {
	if model := config.Model; model != "" {
		req.Model = model
	} else {
		req.Model = p.model
	}
	if maxTokens := config.MaxTokens; maxTokens != nil {
		req.MaxTokens = maxTokens
	} else {
		req.MaxTokens = &p.maxTokens
	}

	if config.ReasoningBudget != nil {
		budget := *config.ReasoningBudget
		if budget < 1024 {
			return fmt.Errorf("reasoning budget must be at least 1024")
		}
		req.Thinking = &Thinking{
			Type:         "enabled",
			BudgetTokens: budget,
		}
	}

	// Compatibility with the OpenAI "effort" parameter
	if config.ReasoningEffort != "" {
		if req.Thinking != nil {
			return fmt.Errorf("cannot set both reasoning budget and effort")
		}
		req.Thinking = &Thinking{Type: "enabled"}
		switch config.ReasoningEffort {
		case llm.ReasoningEffortLow:
			req.Thinking.BudgetTokens = 1024
		case llm.ReasoningEffortMedium:
			req.Thinking.BudgetTokens = 4096
		case llm.ReasoningEffortHigh:
			req.Thinking.BudgetTokens = 16384
		default:
			return fmt.Errorf("invalid reasoning effort: %s", config.ReasoningEffort)
		}
	}

	if len(config.Tools) > 0 {
		var tools []map[string]any
		for _, tool := range config.Tools {
			// Handle tools that explicitly provide a configuration
			if toolWithConfig, ok := tool.(llm.ToolConfiguration); ok {
				toolConfig := toolWithConfig.ToolConfiguration(p.Name())
				// nil means no configuration is specified and to use the default
				if toolConfig != nil {
					tools = append(tools, toolConfig)
					continue
				}
			}
			// Handle tools with the default configuration behavior
			schema := tool.Schema()
			toolConfig := map[string]any{
				"name":        tool.Name(),
				"description": tool.Description(),
			}
			if schema.Type != "" {
				toolConfig["input_schema"] = schema
			}
			tools = append(tools, toolConfig)
		}
		req.Tools = tools
	}

	if config.ToolChoice != nil && len(config.Tools) > 0 {
		req.ToolChoice = &ToolChoice{
			Type: ToolChoiceType(config.ToolChoice.Type),
			Name: config.ToolChoice.Name,
		}
		if config.ParallelToolCalls != nil && !*config.ParallelToolCalls {
			req.ToolChoice.DisableParallelToolUse = true
		}
	}

	if len(config.MCPServers) > 0 {
		req.MCPServers = config.MCPServers
	}

	req.Temperature = config.Temperature
	req.System = config.SystemPrompt
	return nil
}

// createRequest creates an HTTP request with appropriate headers for Anthropic API calls
func (p *Provider) createRequest(ctx context.Context, body []byte, config *llm.Config, isStreaming bool) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", p.endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", p.version)
	req.Header.Set("content-type", "application/json")

	if isStreaming {
		req.Header.Set("accept", "text/event-stream")
	}

	if config.IsFeatureEnabled(FeatureExtendedCache) {
		req.Header.Add("anthropic-beta", FeatureExtendedCache)
	} else if config.IsFeatureEnabled(FeaturePromptCaching) {
		req.Header.Add("anthropic-beta", FeaturePromptCaching)
	} else if config.Caching == nil || *config.Caching {
		req.Header.Add("anthropic-beta", FeaturePromptCaching)
	}

	if config.IsFeatureEnabled(FeatureOutput128k) {
		req.Header.Add("anthropic-beta", FeatureOutput128k)
	}

	if config.IsFeatureEnabled(FeatureMCPClient) || len(config.MCPServers) > 0 {
		req.Header.Add("anthropic-beta", FeatureMCPClient)
	}

	if config.IsFeatureEnabled(FeatureCodeExecution) {
		req.Header.Add("anthropic-beta", FeatureCodeExecution)
	}

	for key, values := range config.RequestHeaders {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	return req, nil
}
