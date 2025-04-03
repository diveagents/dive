package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/slogger"
)

var (
	DefaultResponseTimeout    = time.Minute * 4
	DefaultToolIterationLimit = 16
	ErrThreadsAreNotEnabled   = errors.New("threads are not enabled")
	ErrLLMNoResponse          = errors.New("llm did not return a response")
	ErrNoInstructions         = errors.New("no instructions provided")
	ErrNoLLM                  = errors.New("no llm provided")
	FinishNow                 = "Do not use any more tools. You must respond with your final answer now."
)

// Confirm our standard implementation satisfies the different Agent interfaces
var (
	_ dive.Agent = &Agent{}
)

// ModelSettings are used to configure details of the LLM for an Agent.
type ModelSettings struct {
	Temperature       *float64
	PresencePenalty   *float64
	FrequencyPenalty  *float64
	ReasoningBudget   *int
	ReasoningEffort   string
	MaxTokens         int
	ToolChoice        llm.ToolChoice
	ParallelToolCalls *bool
}

// Options are used to configure an Agent.
type Options struct {
	Name                 string
	Goal                 string
	Backstory            string
	IsSupervisor         bool
	Subordinates         []string
	Model                llm.LLM
	Tools                []llm.Tool
	ToolChoice           llm.ToolChoice
	ResponseTimeout      time.Duration
	Caching              *bool
	Hooks                llm.Hooks
	Logger               slogger.Logger
	ToolIterationLimit   int
	ModelSettings        *ModelSettings
	DateAwareness        *bool
	Environment          dive.Environment
	DocumentRepository   dive.DocumentRepository
	ThreadRepository     dive.ThreadRepository
	SystemPromptTemplate string
}

// Agent is the standard implementation of the Agent interface.
type Agent struct {
	name                 string
	goal                 string
	backstory            string
	model                llm.LLM
	tools                []llm.Tool
	toolsByName          map[string]llm.Tool
	toolChoice           llm.ToolChoice
	isSupervisor         bool
	subordinates         []string
	responseTimeout      time.Duration
	caching              *bool
	hooks                llm.Hooks
	logger               slogger.Logger
	toolIterationLimit   int
	modelSettings        *ModelSettings
	dateAwareness        *bool
	environment          dive.Environment
	documentRepository   dive.DocumentRepository
	threadRepository     dive.ThreadRepository
	systemPromptTemplate *template.Template
	mutex                sync.Mutex
}

// New returns a new Agent configured with the given options.
func New(opts Options) (*Agent, error) {
	if opts.ResponseTimeout <= 0 {
		opts.ResponseTimeout = DefaultResponseTimeout
	}
	if opts.ToolIterationLimit <= 0 {
		opts.ToolIterationLimit = DefaultToolIterationLimit
	}
	if opts.Logger == nil {
		opts.Logger = slogger.DefaultLogger
	}
	if opts.Model == nil {
		if llm, ok := detectProvider(); ok {
			opts.Model = llm
		} else {
			return nil, ErrNoLLM
		}
	}
	if opts.Name == "" {
		opts.Name = dive.RandomName()
	}
	if opts.SystemPromptTemplate == "" {
		opts.SystemPromptTemplate = SystemPromptTemplate
	}
	systemPromptTemplate, err := parseTemplate("agent", opts.SystemPromptTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid system prompt template: %w", err)
	}

	agent := &Agent{
		name:                 strings.TrimSpace(opts.Name),
		goal:                 strings.TrimSpace(opts.Goal),
		backstory:            strings.TrimSpace(opts.Backstory),
		model:                opts.Model,
		environment:          opts.Environment,
		isSupervisor:         opts.IsSupervisor,
		subordinates:         opts.Subordinates,
		responseTimeout:      opts.ResponseTimeout,
		toolIterationLimit:   opts.ToolIterationLimit,
		caching:              opts.Caching,
		hooks:                opts.Hooks,
		logger:               opts.Logger,
		dateAwareness:        opts.DateAwareness,
		documentRepository:   opts.DocumentRepository,
		threadRepository:     opts.ThreadRepository,
		systemPromptTemplate: systemPromptTemplate,
		modelSettings:        opts.ModelSettings,
		toolChoice:           opts.ToolChoice,
	}

	tools := make([]llm.Tool, len(opts.Tools))
	if len(opts.Tools) > 0 {
		copy(tools, opts.Tools)
	}

	// Supervisors need a tool to give work assignments to others
	if opts.IsSupervisor {
		// Only create the assign_work tool if it wasn't provided. This allows
		// a custom assign_work implementation to be used.
		var foundAssignWorkTool bool
		for _, tool := range tools {
			if tool.Definition().Name == "assign_work" {
				foundAssignWorkTool = true
			}
		}
		if !foundAssignWorkTool {
			tools = append(tools, NewAssignWorkTool(AssignWorkToolOptions{
				Self:               agent,
				DefaultTaskTimeout: opts.ResponseTimeout,
			}))
		}
	}

	agent.tools = tools
	if len(tools) > 0 {
		agent.toolsByName = make(map[string]llm.Tool, len(tools))
		for _, tool := range tools {
			agent.toolsByName[tool.Definition().Name] = tool
		}
	}

	// Register with environment if provided
	if opts.Environment != nil {
		if err := opts.Environment.RegisterAgent(agent); err != nil {
			return nil, fmt.Errorf("failed to register agent with environment: %w", err)
		}
	}

	return agent, nil
}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Goal() string {
	return a.goal
}

func (a *Agent) Backstory() string {
	return a.backstory
}

func (a *Agent) IsSupervisor() bool {
	return a.isSupervisor
}

func (a *Agent) Subordinates() []string {
	if !a.isSupervisor || a.environment == nil {
		return nil
	}
	if a.subordinates != nil {
		return a.subordinates
	}
	// If there are no other supervisors, assume we are the supervisor of all
	// agents in the environment.
	var isAnotherSupervisor bool
	for _, agent := range a.environment.Agents() {
		if agent.IsSupervisor() && agent.Name() != a.name {
			isAnotherSupervisor = true
		}
	}
	if isAnotherSupervisor {
		return nil
	}
	var others []string
	for _, agent := range a.environment.Agents() {
		if agent.Name() != a.name {
			others = append(others, agent.Name())
		}
	}
	a.subordinates = others
	return others
}

func (a *Agent) SetEnvironment(env dive.Environment) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.environment != nil {
		return fmt.Errorf("agent is already associated with an environment")
	}
	a.environment = env
	return nil
}

func (a *Agent) CreateResponse(ctx context.Context, opts ...dive.ChatOption) (*dive.Response, error) {
	var chatOptions dive.ChatOptions
	chatOptions.Apply(opts)

	messages := a.prepareMessages(chatOptions)
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	logger := a.logger.With(
		"agent", a.name,
		"thread_id", chatOptions.ThreadID,
		"user_id", chatOptions.UserID,
	)
	logger.Info("creating response")

	// Build the system prompt
	systemPrompt, err := a.buildSystemPrompt("chat")
	if err != nil {
		return nil, fmt.Errorf("failed to build system prompt: %w", err)
	}

	// Append the new messages to the thread history if there is a thread ID
	var thread *dive.Thread
	var threadMessages []*llm.Message
	if chatOptions.ThreadID != "" {
		if a.threadRepository == nil {
			logger.Error("threads are not enabled")
			return nil, ErrThreadsAreNotEnabled
		}
		var err error
		thread, err = a.getOrCreateThread(ctx, chatOptions.ThreadID)
		if err != nil {
			logger.Error("error retrieving thread", "error", err)
			return nil, err
		}
		if len(thread.Messages) > 0 {
			threadMessages = append(threadMessages, thread.Messages...)
		}
	}
	threadMessages = append(threadMessages, messages...)

	// Set a timeout if specified in responseTimeout
	timeoutCtx := ctx
	var cancel context.CancelFunc
	if a.responseTimeout > 0 {
		timeoutCtx, cancel = context.WithTimeout(ctx, a.responseTimeout)
		defer cancel()
	}

	// Generate using the LLM directly
	generateOpts := a.getGenerationOptions(systemPrompt)
	llmResponse, err := a.model.Generate(timeoutCtx, threadMessages, generateOpts...)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Save updated messages to thread if needed
	if thread != nil {
		thread.Messages = append(thread.Messages, llmResponse.Message())
		if err := a.threadRepository.PutThread(ctx, thread); err != nil {
			logger.Error("error saving thread", "error", err)
			return nil, err
		}
	}

	// Convert LLM response to Agent response
	return &dive.Response{
		Text:       llmResponse.Message().Text(),
		Model:      llmResponse.Model,
		TokenUsage: &llmResponse.Usage,
		Raw:        llmResponse,
	}, nil
}

func (a *Agent) StreamResponse(ctx context.Context, opts ...dive.ChatOption) (dive.ResponseStream, error) {
	var chatOptions dive.ChatOptions
	chatOptions.Apply(opts)

	messages := a.prepareMessages(chatOptions)
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	logger := a.logger.With(
		"agent", a.name,
		"thread_id", chatOptions.ThreadID,
		"user_id", chatOptions.UserID,
	)
	logger.Info("streaming response")

	// Create a new event stream to publish events to
	stream, publisher := dive.NewEventStream()

	// Process the streaming response in a goroutine
	go func() {
		defer publisher.Close()

		// Build the system prompt
		systemPrompt, err := a.buildSystemPrompt("chat")
		if err != nil {
			publisher.Send(ctx, a.errorEvent(err))
			return
		}

		// Append the new messages to the thread history if there is a thread ID
		var thread *dive.Thread
		var threadMessages []*llm.Message
		if chatOptions.ThreadID != "" {
			if a.threadRepository == nil {
				logger.Error("threads are not enabled")
				publisher.Send(ctx, a.errorEvent(ErrThreadsAreNotEnabled))
				return
			}
			var err error
			thread, err = a.getOrCreateThread(ctx, chatOptions.ThreadID)
			if err != nil {
				logger.Error("error retrieving thread", "error", err)
				publisher.Send(ctx, a.errorEvent(err))
				return
			}
			if len(thread.Messages) > 0 {
				threadMessages = append(threadMessages, thread.Messages...)
			}
		}
		threadMessages = append(threadMessages, messages...)

		// Set a timeout if specified in responseTimeout
		timeoutCtx := ctx
		var cancel context.CancelFunc
		if a.responseTimeout > 0 {
			timeoutCtx, cancel = context.WithTimeout(ctx, a.responseTimeout)
			defer cancel()
		}

		// Check if model supports streaming
		_, ok := a.model.(llm.StreamingLLM)
		if !ok {
			err := fmt.Errorf("model %s does not support streaming", a.model.Name())
			publisher.Send(ctx, a.errorEvent(err))
			return
		}

		// We'll use the full generate method which handles proper event streaming
		// and tool calls since it already has all that logic
		_, updatedMessages, err := a.generate(timeoutCtx, threadMessages, systemPrompt, publisher)
		if err != nil {
			logger.Error("error generating response", "error", err)
			publisher.Send(ctx, a.errorEvent(err))
			return
		}

		// Save the updated thread messages
		if thread != nil {
			thread.Messages = updatedMessages
			if err := a.threadRepository.PutThread(ctx, thread); err != nil {
				logger.Error("error saving thread", "error", err)
				publisher.Send(ctx, a.errorEvent(err))
				return
			}
		}
	}()

	return stream, nil
}

// prepareMessages processes the ChatOptions to create messages for the LLM.
// It handles both WithMessages and WithInput options.
func (a *Agent) prepareMessages(options dive.ChatOptions) []*llm.Message {
	if len(options.Messages) > 0 {
		return options.Messages
	}

	if options.Input != "" {
		return []*llm.Message{llm.NewUserMessage(options.Input)}
	}

	return nil
}

func (a *Agent) getOrCreateThread(ctx context.Context, threadID string) (*dive.Thread, error) {
	if a.threadRepository == nil {
		return nil, ErrThreadsAreNotEnabled
	}
	thread, err := a.threadRepository.GetThread(ctx, threadID)
	if err == nil {
		return thread, nil
	}
	if err != dive.ErrThreadNotFound {
		return nil, err
	}
	return &dive.Thread{
		ID:       threadID,
		Messages: []*llm.Message{},
	}, nil
}

func (a *Agent) buildSystemPrompt(mode string) (string, error) {
	var responseGuidelines string
	if mode == "task" {
		responseGuidelines = PromptForTaskResponses
	}
	data := newAgentTemplateData(a, responseGuidelines)
	prompt, err := executeTemplate(a.systemPromptTemplate, data)
	if err != nil {
		return "", err
	}
	if a.dateAwareness == nil || *a.dateAwareness {
		prompt = fmt.Sprintf("%s\n\n# Date and Time\n\n%s", prompt, dive.DateString(time.Now()))
	}
	return strings.TrimSpace(prompt), nil
}

// generate runs the LLM generation and tool execution loop. It handles the
// interaction between the agent and the LLM, including tool calls. Returns the
// final LLM response and any error that occurred.
func (a *Agent) generate(
	ctx context.Context,
	messages []*llm.Message,
	systemPrompt string,
	publisher dive.EventPublisher,
) (*llm.Response, []*llm.Message, error) {

	// Holds the most recent response from the LLM
	var response *llm.Response

	// Contains the message history. We'll append to this and return it when done.
	updatedMessages := make([]*llm.Message, len(messages))
	copy(updatedMessages, messages)

	// Options passed to the LLM
	generateOpts := a.getGenerationOptions(systemPrompt)
	generateCfg := &llm.Config{}
	generateCfg.Apply(generateOpts...)

	// Publish an event indicating that generation is starting
	eventOrigin := a.eventOrigin()
	generation := &dive.Generation{
		ID:            dive.NewID(),
		StartedAt:     time.Now(),
		Config:        generateCfg,
		InputMessages: messages,
		Model:         a.model.Name(),
	}
	publisher.Send(ctx, &dive.Event{
		Type:    dive.EventTypeGenerationStarted,
		Origin:  eventOrigin,
		Payload: generation,
	})

	// Helper to keep track of new messages
	addMessage := func(msg *llm.Message) {
		updatedMessages = append(updatedMessages, msg)
		generation.OutputMessages = append(generation.OutputMessages, msg)
	}

	// The loop is used to run and respond to the primary generation request
	// and then automatically run any tool-use invocations. The first time
	// through, we submit the primary generation. On subsequent loops, we are
	// running tool-uses and responding with the results.
	generationLimit := a.toolIterationLimit + 1
	for i := range generationLimit {
		// Generate a response in either streaming or non-streaming mode
		var err error
		if streamingLLM, ok := a.model.(llm.StreamingLLM); ok {
			response, err = a.generateStreaming(ctx, streamingLLM, updatedMessages, generateOpts, publisher)
		} else {
			response, err = a.model.Generate(ctx, updatedMessages, generateOpts...)
		}
		if err == nil && response == nil {
			// This indicates a bug in the LLM provider implementation
			err = ErrLLMNoResponse
		}
		if err != nil {
			now := time.Now()
			generation.Error = err
			generation.CompletedAt = &now
			publisher.Send(ctx, &dive.Event{
				Type:    dive.EventTypeGenerationError,
				Origin:  eventOrigin,
				Payload: generation,
			})
			return nil, nil, err
		}

		a.logger.Debug("llm response",
			"agent", a.name,
			"usage_input_tokens", response.Usage.InputTokens,
			"usage_output_tokens", response.Usage.OutputTokens,
			"cache_creation_input_tokens", response.Usage.CacheCreationInputTokens,
			"cache_read_input_tokens", response.Usage.CacheReadInputTokens,
			"response_text", response.Message().Text(),
			"generation_number", i+1,
		)

		// Remember the assistant response message
		addMessage(response.Message())
		generation.AccumulateUsage(response.Usage)
		generation.ActiveToolCalls = len(response.ToolCalls())

		// We're done if there are no tool calls
		toolCalls := response.ToolCalls()
		if len(toolCalls) == 0 {
			break
		}

		// Publish an event indicating progress
		publisher.Send(ctx, &dive.Event{
			Type:    dive.EventTypeGenerationProgress,
			Origin:  eventOrigin,
			Payload: generation,
		})

		// Execute all requested tool calls
		toolResults, shouldReturnResult, err := a.executeToolCalls(ctx, toolCalls, publisher)
		if err != nil {
			now := time.Now()
			generation.Error = err
			generation.IsDone = true
			generation.CompletedAt = &now
			publisher.Send(ctx, &dive.Event{
				Type:    dive.EventTypeGenerationError,
				Origin:  eventOrigin,
				Payload: generation,
			})
			return nil, nil, err
		}

		// Remember all tool results
		generation.ToolResults = append(generation.ToolResults, toolResults...)

		// We're done if the results don't need to be provided to the LLM
		if !shouldReturnResult {
			break
		}

		// Capture results in a new message to send to LLM on the next iteration
		toolResultMessage := llm.NewToolOutputMessage(toolResults)

		// Add instructions to the message to not use any more tools if we have
		// only one generation left
		if i == generationLimit-2 {
			generateOpts = append(generateOpts, llm.WithToolChoice(llm.ToolChoiceNone))
			a.logger.Debug("set tool choice to none",
				"agent", a.name,
				"generation_number", i+1,
			)
		}
		addMessage(toolResultMessage)
	}

	// Publish an event indicating that the response was fully generated
	now := time.Now()
	generation.CompletedAt = &now
	generation.IsDone = true
	generation.ActiveToolCalls = 0
	publisher.Send(ctx, &dive.Event{
		Type:    dive.EventTypeGenerationCompleted,
		Origin:  eventOrigin,
		Payload: generation,
	})

	return response, updatedMessages, nil
}

// generateStreaming handles streaming generation with an LLM, including
// receiving and republishing events, and accumulating a complete response.
func (a *Agent) generateStreaming(
	ctx context.Context,
	streamingLLM llm.StreamingLLM,
	messages []*llm.Message,
	generateOpts []llm.Option,
	publisher dive.EventPublisher,
) (*llm.Response, error) {
	accum := llm.NewResponseAccumulator()
	iter, err := streamingLLM.Stream(ctx, messages, generateOpts...)
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	for iter.Next() {
		event := iter.Event()
		if err := accum.AddEvent(event); err != nil {
			return nil, err
		}
		publisher.Send(ctx, &dive.Event{
			Type:    dive.EventTypeLLMEvent,
			Origin:  a.eventOrigin(),
			Payload: event,
		})
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return accum.Response(), nil
}

// executeToolCalls executes all tool calls and returns the results. If the
// tools are configured to not return results, (nil, nil) is returned.
func (a *Agent) executeToolCalls(
	ctx context.Context,
	toolCalls []*llm.ToolCall,
	publisher dive.EventPublisher,
) ([]*llm.ToolResult, bool, error) {
	results := make([]*llm.ToolResult, len(toolCalls))
	shouldReturnResult := false
	for i, toolCall := range toolCalls {
		tool, ok := a.toolsByName[toolCall.Name]
		if !ok {
			return nil, false, fmt.Errorf("tool call error: unknown tool %q", toolCall.Name)
		}
		a.logger.Debug("executing tool call",
			"tool_id", toolCall.ID,
			"tool_name", toolCall.Name,
			"tool_input", toolCall.Input)

		now := time.Now()
		result := &llm.ToolResult{
			ID:        toolCall.ID,
			Name:      toolCall.Name,
			Input:     toolCall.Input,
			StartedAt: &now,
		}
		publisher.Send(ctx, &dive.Event{
			Type:    dive.EventTypeToolCalled,
			Origin:  a.eventOrigin(),
			Payload: result,
		})

		output, err := tool.Call(ctx, toolCall.Input)
		finishedAt := time.Now()
		result.CompletedAt = &finishedAt
		if err != nil {
			result.Error = err
			publisher.Send(ctx, &dive.Event{
				Type:    dive.EventTypeToolError,
				Origin:  a.eventOrigin(),
				Payload: result,
			})
			return nil, false, fmt.Errorf("tool call error: %w", err)
		}
		result.Output = output
		results[i] = result

		publisher.Send(ctx, &dive.Event{
			Type:    dive.EventTypeToolOutput,
			Origin:  a.eventOrigin(),
			Payload: result,
		})

		if tool.ShouldReturnResult() {
			shouldReturnResult = true
		}
	}
	if shouldReturnResult {
		return results, true, nil
	}
	return results, false, nil
}

func (a *Agent) getGenerationOptions(systemPrompt string) []llm.Option {
	var generateOpts []llm.Option
	if systemPrompt != "" {
		generateOpts = append(generateOpts, llm.WithSystemPrompt(systemPrompt))
	}
	if len(a.tools) > 0 {
		generateOpts = append(generateOpts, llm.WithTools(a.tools...))
	}
	if a.toolChoice != "" {
		generateOpts = append(generateOpts, llm.WithToolChoice(a.toolChoice))
	}
	if a.hooks != nil {
		generateOpts = append(generateOpts, llm.WithHooks(a.hooks))
	}
	if a.logger != nil {
		generateOpts = append(generateOpts, llm.WithLogger(a.logger))
	}
	if a.caching != nil {
		generateOpts = append(generateOpts, llm.WithCaching(*a.caching))
	} else {
		// Caching defaults to on
		generateOpts = append(generateOpts, llm.WithCaching(true))
	}
	if a.modelSettings != nil {
		settings := a.modelSettings
		if settings.Temperature != nil {
			generateOpts = append(generateOpts, llm.WithTemperature(*settings.Temperature))
		}
		if settings.PresencePenalty != nil {
			generateOpts = append(generateOpts, llm.WithPresencePenalty(*settings.PresencePenalty))
		}
		if settings.FrequencyPenalty != nil {
			generateOpts = append(generateOpts, llm.WithFrequencyPenalty(*settings.FrequencyPenalty))
		}
		if settings.ReasoningBudget != nil {
			generateOpts = append(generateOpts, llm.WithReasoningBudget(*settings.ReasoningBudget))
		}
		if settings.ReasoningEffort != "" {
			generateOpts = append(generateOpts, llm.WithReasoningEffort(settings.ReasoningEffort))
		}
		if settings.MaxTokens != 0 {
			generateOpts = append(generateOpts, llm.WithMaxTokens(settings.MaxTokens))
		}
		if settings.ToolChoice != "" {
			generateOpts = append(generateOpts, llm.WithToolChoice(settings.ToolChoice))
		}
		if settings.ParallelToolCalls != nil {
			generateOpts = append(generateOpts, llm.WithParallelToolCalls(*settings.ParallelToolCalls))
		}
	}
	return generateOpts
}

func (a *Agent) TeamOverview() string {
	if a.environment == nil {
		return ""
	}
	agents := a.environment.Agents()
	if len(agents) == 0 {
		return ""
	}
	if len(agents) == 1 && agents[0].Name() == a.name {
		return "You are the only agent on the team."
	}
	lines := []string{
		"The team is comprised of the following agents:",
	}
	for _, agent := range agents {
		description := fmt.Sprintf("- Name: %s", agent.Name())
		if goal := agent.Goal(); goal != "" {
			description += fmt.Sprintf(" Goal: %s", goal)
		}
		if agent.Name() == a.name {
			description += " (You)"
		}
		lines = append(lines, description)
	}
	return strings.Join(lines, "\n")
}

func (a *Agent) environmentName() string {
	if a.environment == nil {
		return ""
	}
	return a.environment.Name()
}

func (a *Agent) eventOrigin() dive.EventOrigin {
	var environmentName string
	if a.environment != nil {
		environmentName = a.environment.Name()
	}
	return dive.EventOrigin{
		AgentName:       a.name,
		EnvironmentName: environmentName,
	}
}

func (a *Agent) errorEvent(err error) *dive.Event {
	return &dive.Event{
		Type:   dive.EventTypeError,
		Error:  err,
		Origin: a.eventOrigin(),
	}
}

func taskPromptMessages(prompt *dive.Prompt) ([]*llm.Message, error) {
	messages := []*llm.Message{}

	if prompt.Text == "" {
		return nil, ErrNoInstructions
	}

	// Add context information if available
	if len(prompt.Context) > 0 {
		contextLines := []string{
			"Important: The following context may contain relevant information to help you complete the task.",
		}
		for _, context := range prompt.Context {
			var contextBlock string
			if context.Name != "" {
				contextBlock = fmt.Sprintf("<context name=%q>\n%s\n</context>", context.Name, context.Text)
			} else {
				contextBlock = fmt.Sprintf("<context>\n%s\n</context>", context.Text)
			}
			contextLines = append(contextLines, contextBlock)
		}
		messages = append(messages, llm.NewUserMessage(strings.Join(contextLines, "\n\n")))
	}

	var lines []string

	// Add task instructions
	lines = append(lines, "You must complete the following task:")
	if prompt.Name != "" {
		lines = append(lines, fmt.Sprintf("<task name=%q>\n%s\n</task>", prompt.Name, prompt.Text))
	} else {
		lines = append(lines, fmt.Sprintf("<task>\n%s\n</task>", prompt.Text))
	}

	// Add output expectations if specified
	if prompt.Output != "" {
		output := "Response requirements: " + prompt.Output
		if prompt.OutputFormat != "" {
			output += fmt.Sprintf("\n\nFormat your response in %s format.", prompt.OutputFormat)
		}
		lines = append(lines, output)
	}

	messages = append(messages, llm.NewUserMessage(strings.Join(lines, "\n\n")))
	return messages, nil
}
