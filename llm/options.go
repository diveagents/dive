package llm

import (
	"context"
	"net/http"

	"github.com/diveagents/dive/slogger"
)

// Option is a function that is used to adjust LLM configuration.
type Option func(*Config)

// Config is used to configure LLM calls. Not all providers support all options.
// If a provider doesn't support a given option, it will be ignored.
type Config struct {
	Model              string                   `json:"model,omitempty"`
	SystemPrompt       string                   `json:"system_prompt,omitempty"`
	Endpoint           string                   `json:"endpoint,omitempty"`
	APIKey             string                   `json:"api_key,omitempty"`
	Prefill            string                   `json:"prefill,omitempty"`
	PrefillClosingTag  string                   `json:"prefill_closing_tag,omitempty"`
	MaxTokens          *int                     `json:"max_tokens,omitempty"`
	Temperature        *float64                 `json:"temperature,omitempty"`
	PresencePenalty    *float64                 `json:"presence_penalty,omitempty"`
	FrequencyPenalty   *float64                 `json:"frequency_penalty,omitempty"`
	ReasoningBudget    *int                     `json:"reasoning_budget,omitempty"`
	ReasoningEffort    ReasoningEffort          `json:"reasoning_effort,omitempty"`
	ReasoningSummary   ReasoningSummary         `json:"reasoning_summary,omitempty"`
	Tools              []Tool                   `json:"tools,omitempty"`
	ToolChoice         *ToolChoice              `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool                    `json:"parallel_tool_calls,omitempty"`
	Features           []string                 `json:"features,omitempty"`
	RequestHeaders     http.Header              `json:"request_headers,omitempty"`
	MCPServers         []MCPServerConfig        `json:"mcp_servers,omitempty"`
	Caching            *bool                    `json:"caching,omitempty"`
	PreviousResponseID string                   `json:"previous_response_id,omitempty"`
	ServiceTier        string                   `json:"service_tier,omitempty"`
	ProviderOptions    map[string]interface{}   `json:"provider_options,omitempty"`
	ResponseFormat     *ResponseFormat          `json:"response_format,omitempty"`
	Messages           Messages                 `json:"messages"`
	Hooks              Hooks                    `json:"-"`
	Client             *http.Client             `json:"-"`
	Logger             slogger.Logger           `json:"-"`
	SSECallback        ServerSentEventsCallback `json:"-"`
}

// Apply applies the given options to the config.
func (c *Config) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(c)
	}
}

// FireHooks fires the configured hooks for the matching hook type.
func (c *Config) FireHooks(ctx context.Context, hookCtx *HookContext) error {
	for _, hook := range c.Hooks {
		if hook.Type == hookCtx.Type {
			if err := hook.Func(ctx, hookCtx); err != nil {
				return err
			}
		}
	}
	return nil
}

// IsFeatureEnabled returns true if the feature is enabled.
func (c *Config) IsFeatureEnabled(feature string) bool {
	for _, f := range c.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// WithModel sets the LLM model for the generation.
func WithModel(model string) Option {
	return func(config *Config) {
		config.Model = model
	}
}

// WithLogger sets the logger.
func WithLogger(logger slogger.Logger) Option {
	return func(config *Config) {
		config.Logger = logger
	}
}

// WithMaxTokens sets the max tokens.
func WithMaxTokens(maxTokens int) Option {
	return func(config *Config) {
		config.MaxTokens = &maxTokens
	}
}

// WithEndpoint sets the endpoint.
func WithEndpoint(endpoint string) Option {
	return func(config *Config) {
		config.Endpoint = endpoint
	}
}

// WithHTTPClient sets the HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(config *Config) {
		config.Client = client
	}
}

// WithTemperature sets the temperature.
func WithTemperature(temperature float64) Option {
	return func(config *Config) {
		config.Temperature = &temperature
	}
}

// WithSystemPrompt sets the system prompt.
func WithSystemPrompt(systemPrompt string) Option {
	return func(config *Config) {
		config.SystemPrompt = systemPrompt
	}
}

// WithTools sets the tools for the interaction.
func WithTools(tools ...Tool) Option {
	return func(config *Config) {
		config.Tools = tools
	}
}

// WithToolChoice sets the tool choice for the interaction.
func WithToolChoice(toolChoice *ToolChoice) Option {
	return func(config *Config) {
		config.ToolChoice = toolChoice
	}
}

// WithParallelToolCalls sets whether to allow parallel tool calls.
func WithParallelToolCalls(parallelToolCalls bool) Option {
	return func(config *Config) {
		config.ParallelToolCalls = &parallelToolCalls
	}
}

// WithHook adds a callback for the specified hook type
func WithHook(hookType HookType, hookFunc HookFunc) Option {
	return func(config *Config) {
		config.Hooks = append(config.Hooks, Hook{
			Type: hookType,
			Func: hookFunc,
		})
	}
}

// WithHooks sets the hooks for the interaction.
func WithHooks(hooks Hooks) Option {
	return func(config *Config) {
		config.Hooks = hooks
	}
}

// WithAPIKey sets the API key.
func WithAPIKey(apiKey string) Option {
	return func(config *Config) {
		config.APIKey = apiKey
	}
}

// WithPrefill sets the prefilled assistant response for the interaction.
func WithPrefill(prefill, closingTag string) Option {
	return func(config *Config) {
		config.Prefill = prefill
		config.PrefillClosingTag = closingTag
	}
}

// WithPresencePenalty sets the presence penalty for the interaction.
func WithPresencePenalty(presencePenalty float64) Option {
	return func(config *Config) {
		config.PresencePenalty = &presencePenalty
	}
}

// WithFrequencyPenalty sets the frequency penalty for the interaction.
func WithFrequencyPenalty(frequencyPenalty float64) Option {
	return func(config *Config) {
		config.FrequencyPenalty = &frequencyPenalty
	}
}

// WithReasoningBudget sets the reasoning budget for the interaction.
func WithReasoningBudget(reasoningBudget int) Option {
	return func(config *Config) {
		config.ReasoningBudget = &reasoningBudget
	}
}

// ReasoningEffort defines the effort level for reasoning aka extended thinking.
type ReasoningEffort string

const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
)

// IsValid returns true if the reasoning effort is a known, valid value.
func (r ReasoningEffort) IsValid() bool {
	return r == ReasoningEffortLow ||
		r == ReasoningEffortMedium ||
		r == ReasoningEffortHigh
}

// WithReasoningEffort sets the reasoning effort for the interaction.
func WithReasoningEffort(reasoningEffort ReasoningEffort) Option {
	return func(config *Config) {
		config.ReasoningEffort = reasoningEffort
	}
}

type ReasoningSummary string

const (
	ReasoningSummaryAuto     ReasoningSummary = "auto"
	ReasoningSummaryConcise  ReasoningSummary = "concise"
	ReasoningSummaryDetailed ReasoningSummary = "detailed"
)

// WithReasoningSummary sets the reasoning summary for the interaction.
func WithReasoningSummary(reasoningSummary ReasoningSummary) Option {
	return func(config *Config) {
		config.ReasoningSummary = reasoningSummary
	}
}

// WithFeatures sets the features for the interaction.
func WithFeatures(features ...string) Option {
	return func(config *Config) {
		config.Features = append(config.Features, features...)
	}
}

// WithMessages sets the messages for the interaction.
func WithMessages(messages ...*Message) Option {
	return func(config *Config) {
		config.Messages = messages
	}
}

// WithUserTextMessage sets a single user text message for the interaction.
func WithUserTextMessage(text string) Option {
	return func(config *Config) {
		config.Messages = Messages{NewUserTextMessage(text)}
	}
}

// WithRequestHeaders sets the request headers for the interaction.
func WithRequestHeaders(headers http.Header) Option {
	return func(config *Config) {
		config.RequestHeaders = headers
	}
}

// WithMCPServers sets the remote MCP servers for the interaction.
// Used to configure MCP servers that the LLM provider itself is going to call.
// Corresponds to the Anthropic "MCP connector" feature:
// https://docs.anthropic.com/en/docs/agents-and-tools/mcp-connector
func WithMCPServers(servers ...MCPServerConfig) Option {
	return func(config *Config) {
		config.MCPServers = append(config.MCPServers, servers...)
	}
}

// WithResponseFormat sets the response format for the interaction.
func WithResponseFormat(responseFormat *ResponseFormat) Option {
	return func(config *Config) {
		config.ResponseFormat = responseFormat
	}
}

// WithPreviousResponseID sets the previous response ID for the interaction.
// OpenAI only.
// https://platform.openai.com/docs/guides/conversation-state?api-mode=responses#openai-apis-for-conversation-state
func WithPreviousResponseID(previousResponseID string) Option {
	return func(config *Config) {
		config.PreviousResponseID = previousResponseID
	}
}

// WithServiceTier sets the service tier for the interaction.
// OpenAI only.
// https://platform.openai.com/docs/api-reference/responses/create#responses-create-service_tier
func WithServiceTier(serviceTier string) Option {
	return func(config *Config) {
		config.ServiceTier = serviceTier
	}
}

// WithServerSentEventsCallback sets the callback for the server-sent events stream.
func WithServerSentEventsCallback(callback ServerSentEventsCallback) Option {
	return func(config *Config) {
		config.SSECallback = callback
	}
}
