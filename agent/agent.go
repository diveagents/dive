package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/events"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/memory"
	"github.com/getstingrai/dive/slogger"
)

var (
	DefaultStepTimeout        = time.Minute * 5
	DefaultChatTimeout        = time.Minute * 1
	DefaultTickFrequency      = time.Second * 1
	DefaultToolIterationLimit = 8
)

// Confirm our standard implementation satisfies the different Agent interfaces
var (
	_ dive.Agent             = &Agent{}
	_ dive.RunnableAgent     = &Agent{}
	_ dive.EventHandlerAgent = &Agent{}
)

// chatThread contains the message history for a specific chat thread ID
type chatThread struct {
	ID       string
	Messages []*llm.Message
}

// AgentOptions are used to configure an Agent.
type AgentOptions struct {
	Name               string
	Description        string
	Instructions       string
	AcceptedEvents     []string
	IsSupervisor       bool
	Subordinates       []string
	LLM                llm.LLM
	Tools              []llm.Tool
	TickFrequency      time.Duration
	StepTimeout        time.Duration
	ChatTimeout        time.Duration
	CacheControl       string
	LogLevel           string
	Hooks              llm.Hooks
	Logger             slogger.Logger
	ToolIterationLimit int
	Memory             memory.Memory
	Temperature        *float64
	PresencePenalty    *float64
	FrequencyPenalty   *float64
	ReasoningFormat    string
	ReasoningEffort    string
	DateAwareness      *bool
	Environment        dive.Environment
}

// Agent is the standard implementation of the Agent interface.
type Agent struct {
	name         string
	description  string
	instructions string
	llm          llm.LLM
	// team               dive.Team
	running            bool
	tools              []llm.Tool
	toolsByName        map[string]llm.Tool
	acceptedEvents     []string
	isSupervisor       bool
	subordinates       []string
	tickFrequency      time.Duration
	taskTimeout        time.Duration
	chatTimeout        time.Duration
	cacheControl       string
	taskQueue          []*taskState
	recentTasks        []*taskState
	activeTask         *taskState
	ticker             *time.Ticker
	logLevel           string
	hooks              llm.Hooks
	logger             slogger.Logger
	toolIterationLimit int
	memory             memory.Memory
	threads            map[string]*chatThread
	temperature        *float64
	presencePenalty    *float64
	frequencyPenalty   *float64
	reasoningFormat    string
	reasoningEffort    string
	dateAwareness      *bool
	environment        dive.Environment

	// Holds incoming messages to be processed by the agent's run loop
	mailbox chan interface{}

	mutex sync.Mutex
	wg    sync.WaitGroup
}

// NewAgent returns a new Agent configured with the given options.
func NewAgent(opts AgentOptions) *Agent {
	if opts.TickFrequency <= 0 {
		opts.TickFrequency = DefaultTickFrequency
	}
	if opts.StepTimeout <= 0 {
		opts.StepTimeout = DefaultStepTimeout
	}
	if opts.ChatTimeout <= 0 {
		opts.ChatTimeout = DefaultChatTimeout
	}
	if opts.ToolIterationLimit <= 0 {
		opts.ToolIterationLimit = DefaultToolIterationLimit
	}
	if opts.Logger == nil {
		opts.Logger = dive.DefaultLogger
	}
	if opts.LLM == nil {
		if llm, ok := detectProvider(); ok {
			opts.LLM = llm
		} else {
			panic("no llm provided")
		}
	}
	if opts.Name == "" {
		if opts.Description != "" {
			opts.Name = opts.Description
		} else {
			opts.Name = randomName()
		}
	}
	agent := &Agent{
		name:               opts.Name,
		llm:                opts.LLM,
		description:        opts.Description,
		instructions:       opts.Instructions,
		environment:        opts.Environment,
		acceptedEvents:     opts.AcceptedEvents,
		isSupervisor:       opts.IsSupervisor,
		subordinates:       opts.Subordinates,
		tickFrequency:      opts.TickFrequency,
		taskTimeout:        opts.StepTimeout,
		chatTimeout:        opts.ChatTimeout,
		toolIterationLimit: opts.ToolIterationLimit,
		cacheControl:       opts.CacheControl,
		hooks:              opts.Hooks,
		mailbox:            make(chan interface{}, 16),
		logger:             opts.Logger,
		logLevel:           strings.ToLower(opts.LogLevel),
		threads:            make(map[string]*chatThread),
		dateAwareness:      opts.DateAwareness,
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
				DefaultStepTimeout: opts.StepTimeout,
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
			panic(fmt.Sprintf("failed to register agent with environment: %v", err))
		}
	}

	return agent
}

func (a *Agent) Name() string {
	return a.name
}

func (a *Agent) Description() string {
	return a.description
}

func (a *Agent) Instructions() string {
	return a.instructions
}

func (a *Agent) AcceptedEvents() []string {
	return a.acceptedEvents
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

// func (a *Agent) Join(team Team) error {
// 	a.mutex.Lock()
// 	defer a.mutex.Unlock()

// 	if a.running {
// 		return fmt.Errorf("agent is already running")
// 	}
// 	if a.team != nil {
// 		return fmt.Errorf("agent is already a member of a team")
// 	}
// 	a.team = team
// 	return nil
// }

// func (a *Agent) Team() Team {
// 	return a.team
// }

func (a *Agent) Start(ctx context.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.running {
		return fmt.Errorf("agent is already running")
	}

	a.running = true
	a.wg = sync.WaitGroup{}
	a.wg.Add(1)
	go a.run()

	a.logger.Debug("agent started",
		"name", a.name,
		"description", a.description,
		"cache_control", a.cacheControl,
		"is_supervisor", a.isSupervisor,
		"subordinates", a.subordinates,
		"task_timeout", a.taskTimeout,
		"chat_timeout", a.chatTimeout,
		"tick_frequency", a.tickFrequency,
		"tool_iteration_limit", a.toolIterationLimit,
		"model", a.llm.Name(),
	)
	return nil
}

func (a *Agent) Stop(ctx context.Context) error {
	a.mutex.Lock()
	defer func() {
		a.running = false
		a.mutex.Unlock()
		a.logger.Debug("agent stopped", "name", a.name)
	}()

	if !a.running {
		return fmt.Errorf("agent is not running")
	}
	done := make(chan error)

	a.mailbox <- messageStop{ctx: ctx, done: done}
	close(a.mailbox)

	select {
	case err := <-done:
		a.wg.Wait()
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for agent to stop: %w", ctx.Err())
	}
}

func (a *Agent) IsRunning() bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.running
}

func (a *Agent) Generate(ctx context.Context, message *llm.Message, opts ...GenerateOption) (*llm.Response, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	var generateOptions generateOptions
	for _, opt := range opts {
		opt(&generateOptions)
	}

	resultChan := make(chan *llm.Response, 1)
	errChan := make(chan error, 1)

	chatMessage := messageChat{
		message:    message,
		options:    generateOptions,
		resultChan: resultChan,
		errChan:    errChan,
	}

	// Send the chat message to the agent's mailbox, but make sure we timeout
	// if the agent doesn't pick it up in a reasonable amount of time
	select {
	case a.mailbox <- chatMessage:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Wait for the agent to respond
	select {
	case resp := <-resultChan:
		return resp, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *Agent) Stream(ctx context.Context, message *llm.Message, opts ...GenerateOption) (events.Stream, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	var generateOptions generateOptions
	generateOptions.Apply(opts)

	stream := events.NewStream()

	chatMessage := messageChat{
		message: message,
		options: generateOptions,
		stream:  stream,
	}

	// Send the chat message to the agent's mailbox, but make sure we timeout
	// if the agent doesn't pick it up in a reasonable amount of time
	select {
	case a.mailbox <- chatMessage:
	case <-ctx.Done():
		stream.Close()
		return nil, ctx.Err()
	}

	return stream, nil
}

func (a *Agent) HandleEvent(ctx context.Context, event *events.Event) error {
	if !a.IsRunning() {
		return fmt.Errorf("agent is not running")
	}

	select {
	case a.mailbox <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Agent) Work(ctx context.Context, task dive.Task) (events.Stream, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	// Stream to be returned to the caller so it can wait for results
	stream := events.NewStream()

	message := messageWork{
		task:      task,
		publisher: stream.Publisher(),
	}

	select {
	case a.mailbox <- message:
		return stream, nil
	case <-ctx.Done():
		stream.Close()
		return nil, ctx.Err()
	}
}

// This is the agent's main run loop. It dispatches incoming messages and runs
// a ticker that wakes the agent up periodically even if there are no messages.
func (a *Agent) run() error {
	defer a.wg.Done()

	a.ticker = time.NewTicker(a.tickFrequency)
	defer a.ticker.Stop()

	for {
		select {
		case <-a.ticker.C:
		case msg := <-a.mailbox:
			switch msg := msg.(type) {
			case messageWork:
				a.handleWork(msg)

			case messageChat:
				a.handleChat(msg)

			case *events.Event:
				a.handleEvent(msg)

			case messageStop:
				msg.done <- nil
				return nil
			}
		}
		// Make progress on any active tasks
		a.doSomeWork()
	}
}

func (a *Agent) handleWork(m messageWork) {
	a.taskQueue = append(a.taskQueue, &taskState{
		Task:      m.task,
		Publisher: m.publisher,
		Status:    dive.TaskStatusQueued,
	})
}

func (a *Agent) handleEvent(event *events.Event) {
	a.logger.Info("event received",
		"agent", a.name,
		"event_type", event.Type)

	// TODO: implement event triggered behaviors
}

func (a *Agent) handleChat(m messageChat) {
	ctx, cancel := context.WithTimeout(context.Background(), a.chatTimeout)
	defer cancel()

	systemPrompt, err := a.getSystemPromptForMode("chat")
	if err != nil {
		m.errChan <- err
		return
	}

	var isStreaming bool
	var publisher events.Publisher
	if m.stream != nil {
		isStreaming = true
		publisher = m.stream.Publisher()
		defer publisher.Close()
	}

	logger := a.logger.With(
		"agent_name", a.name,
		"streaming", isStreaming,
		"thread_id", m.options.ThreadID,
		"user_id", m.options.UserID,
	)

	logger.Info("handling chat",
		"truncated_message", TruncateText(m.message.Text(), 10))

	// Append this new message to the thread history if a thread ID is provided
	var thread *chatThread
	var messages []*llm.Message
	if m.options.ThreadID != "" {
		var exists bool
		thread, exists = a.threads[m.options.ThreadID]
		if !exists {
			thread = &chatThread{
				ID:       m.options.ThreadID,
				Messages: []*llm.Message{},
			}
			a.threads[m.options.ThreadID] = thread
		}
		messages = append(messages, thread.Messages...)
	}
	messages = append(messages, m.message)

	response, updatedMessages, err := a.generate(
		ctx,
		messages,
		systemPrompt,
		"chat",
		publisher,
	)
	if err != nil {
		logger.Error("error generating chat response", "error", err)
		// Intentional fall-through
	}
	if thread != nil {
		thread.Messages = updatedMessages
	}

	if isStreaming {
		return
	}
	if err != nil {
		m.errChan <- err
		return
	}
	m.resultChan <- response
}

func (a *Agent) getSystemPromptForMode(mode string) (string, error) {
	var err error
	var prompt string
	data := newAgentTemplateData(a)
	if mode == "chat" {
		prompt, err = executeTemplate(chatSystemPromptTemplate, data)
	} else {
		prompt, err = executeTemplate(agentSystemPromptTemplate, data)
	}
	if err != nil {
		return "", err
	}
	if a.dateAwareness == nil || *a.dateAwareness {
		prompt = fmt.Sprintf("%s\n\n%s", prompt, dateString(time.Now()))
	}
	return prompt, nil
}

// generate runs the LLM generation and tool execution loop.
// It handles the interaction between the agent and the LLM, including tool calls.
// Returns the final LLM response and any error that occurred.
func (a *Agent) generate(
	ctx context.Context,
	messages []*llm.Message,
	systemPrompt string,
	stepName string,
	publisher events.Publisher,
) (*llm.Response, []*llm.Message, error) {

	// Holds the most recent response from the LLM
	var response *llm.Response
	updatedMessages := make([]*llm.Message, len(messages))
	copy(updatedMessages, messages)

	// Helper function to safely send events to the publisher
	safePublish := func(event *events.Event) error {
		if publisher == nil {
			return nil
		}
		return publisher.Send(ctx, event)
	}

	generationLimit := a.toolIterationLimit + 1

	// The loop is used to run and respond to the primary generation request
	// and then automatically run any tool-use invocations. The first time
	// through, we submit the primary generation. On subsequent loops, we are
	// running tool-uses and responding with the results.
	for i := 0; i < generationLimit; i++ {
		generateOpts := []llm.Option{
			llm.WithSystemPrompt(systemPrompt),
			llm.WithCacheControl(a.cacheControl),
			llm.WithLogLevel(a.logLevel),
			llm.WithTools(a.tools...),
		}
		if a.hooks != nil {
			generateOpts = append(generateOpts, llm.WithHooks(a.hooks))
		}
		if a.logger != nil {
			generateOpts = append(generateOpts, llm.WithLogger(a.logger))
		}
		if a.temperature != nil {
			generateOpts = append(generateOpts, llm.WithTemperature(*a.temperature))
		}
		if a.presencePenalty != nil {
			generateOpts = append(generateOpts, llm.WithPresencePenalty(*a.presencePenalty))
		}
		if a.frequencyPenalty != nil {
			generateOpts = append(generateOpts, llm.WithFrequencyPenalty(*a.frequencyPenalty))
		}
		if a.reasoningFormat != "" {
			generateOpts = append(generateOpts, llm.WithReasoningFormat(a.reasoningFormat))
		}
		if a.reasoningEffort != "" {
			generateOpts = append(generateOpts, llm.WithReasoningEffort(a.reasoningEffort))
		}

		var currentResponse *llm.Response

		if streamingLLM, ok := a.llm.(llm.StreamingLLM); ok {
			stream, err := streamingLLM.Stream(ctx, updatedMessages, generateOpts...)
			if err != nil {
				return nil, updatedMessages, err
			}
			// Guarantee that Close is called. It's ok if this is redundant with
			// additional calls to Close below.
			defer stream.Close()

			for {
				event, ok := stream.Next(ctx)
				if !ok {
					if err := stream.Err(); err != nil {
						return nil, updatedMessages, err
					}
					break
				}
				if event.Response != nil {
					currentResponse = event.Response
				}
				err = safePublish(&events.Event{
					Type: "llm.event",
					// StepName:  stepName,
					AgentName: a.name,
					LLMEvent:  event,
					Response:  currentResponse,
				})
				if err != nil {
					return nil, updatedMessages, err
				}
			}
			stream.Close()
		} else {
			var err error
			currentResponse, err = a.llm.Generate(ctx, updatedMessages, generateOpts...)
			if err != nil {
				return nil, updatedMessages, err
			}
		}

		if currentResponse == nil {
			// This indicates a bug in the LLM provider implementation
			return nil, updatedMessages, errors.New("no final response from llm provider")
		}
		response = currentResponse
		responseMessage := response.Message()

		a.logger.Info("llm response",
			"usage_input_tokens", response.Usage().InputTokens,
			"usage_output_tokens", response.Usage().OutputTokens,
			"cache_creation_input_tokens", response.Usage().CacheCreationInputTokens,
			"cache_read_input_tokens", response.Usage().CacheReadInputTokens,
			"truncated_response", TruncateText(responseMessage.Text(), 10),
			"generation_number", i+1,
		)

		// Remember the assistant response message
		updatedMessages = append(updatedMessages, responseMessage)

		// We're done if there are no tool-uses
		if len(response.ToolCalls()) == 0 {
			break
		}

		// Execute all requested tool uses and accumulate results
		shouldReturnResult := false
		toolResults := make([]*llm.ToolResult, len(response.ToolCalls()))
		for i, toolCall := range response.ToolCalls() {
			tool, ok := a.toolsByName[toolCall.Name]
			if !ok {
				return nil, updatedMessages, fmt.Errorf("tool call for unknown tool: %q", toolCall.Name)
			}
			a.logger.Debug(
				"tool call",
				"agent_name", a.name,
				"tool_name", toolCall.Name,
				"tool_input", toolCall.Input,
			)
			result, err := tool.Call(ctx, toolCall.Input)
			if err != nil {
				return nil, updatedMessages, fmt.Errorf("tool call error: %w", err)
			}
			toolResults[i] = &llm.ToolResult{
				ID:     toolCall.ID,
				Name:   toolCall.Name,
				Result: result,
			}
			if tool.ShouldReturnResult() {
				shouldReturnResult = true
			}
		}

		// If no tool calls need to return results to the LLM, we're done
		if !shouldReturnResult {
			break
		}
		// Capture results in a new message to send on next loop iteration
		resultMessage := llm.NewToolResultMessage(toolResults)

		// Add instructions to the message to not use any more tools if we have
		// only one generation left.
		if i == generationLimit-2 {
			resultMessage.Content = append(resultMessage.Content, &llm.Content{
				Type: llm.ContentTypeText,
				Text: "Do not use any more tools. You must respond with your final answer now.",
			})
			a.logger.Debug(
				"adding tool use limit instruction",
				"agent", a.name,
				"step", stepName,
				"generation_number", i+1,
			)
		}
		updatedMessages = append(updatedMessages, resultMessage)
	}

	return response, updatedMessages, nil
}

func (a *Agent) documentStore() document.Repository {
	return a.environment.Repository()
}

func (a *Agent) getTaskDocumentsMessage(ctx context.Context, task dive.Task) (*llm.Message, error) {
	documents, err := a.loadTaskDocuments(ctx, task)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, nil
	}
	var parts []string
	for _, doc := range documents {
		text := fmt.Sprintf("<document id=%q name=%q>\n%s\n</document>",
			doc.ID(), doc.Name(), doc.Content())
		parts = append(parts, text)
	}
	return llm.NewUserMessage(strings.Join(parts, "\n\n")), nil
}

// loadTaskDocuments loads the content of documents referenced by a task
func (a *Agent) loadTaskDocuments(ctx context.Context, task dive.Task) ([]document.Document, error) {
	// if len(task.DocumentRefs()) == 0 {
	// 	return nil, nil
	// }
	// var documents []document.Document
	// for _, ref := range task.DocumentRefs() {
	// 	var err error
	// 	var doc document.Document
	// 	if ref.Name != "" {
	// 		doc, err = a.documentStore().GetDocument(ctx, ref.Name)
	// 		if err != nil {
	// 			return nil, fmt.Errorf("document with name %q not found", ref.Name)
	// 		}
	// 	} else if ref.Glob != "" {
	// 		// Validate glob pattern can be used as path prefix
	// 		if !strings.HasSuffix(ref.Glob, "/*") || strings.Contains(ref.Glob[:len(ref.Glob)-2], "*") {
	// 			return nil, fmt.Errorf("invalid glob pattern %q - only trailing '/*' is supported", ref.Glob)
	// 		}
	// 		// Convert glob to path prefix by removing the trailing "/*"
	// 		pathPrefix := ref.Glob[:len(ref.Glob)-2]
	// 		docs, err := a.documentStore().ListDocuments(ctx, &document.ListDocumentInput{
	// 			PathPrefix: pathPrefix,
	// 		})
	// 		if err != nil {
	// 			return nil, fmt.Errorf("error listing documents with path prefix %q", pathPrefix)
	// 		}
	// 		documents = append(documents, docs.Items...)
	// 	} else {
	// 		return nil, fmt.Errorf("unsupported document reference: %q", ref)
	// 	}
	// 	documents = append(documents, doc)
	// }
	// if len(documents) == 0 {
	// 	return nil, fmt.Errorf("no documents found for step %q", step.Name())
	// }
	var documents []document.Document
	return documents, nil
}

func (a *Agent) handleTask(ctx context.Context, state *taskState) error {
	task := state.Task

	var timeout time.Duration // := task.Timeout()
	if timeout == 0 {
		timeout = a.taskTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	logger := a.logger.With(
		"agent_name", a.name,
		"task_name", task.Name(),
		"timeout", timeout.String(),
	)

	systemPrompt, err := a.getSystemPromptForMode("task")
	if err != nil {
		return err
	}

	documentsMessage, err := a.getTaskDocumentsMessage(ctx, task)
	if err != nil {
		return err
	}

	messages := []*llm.Message{}

	if len(state.Messages) == 0 {
		// Starting a step
		if recentTasksMessage, ok := a.getTasksHistoryMessage(); ok {
			messages = append(messages, recentTasksMessage)
		}
		if documentsMessage != nil {
			messages = append(messages, documentsMessage)
		}
		messages = append(messages, llm.NewUserMessage(task.Prompt(dive.TaskPromptOptions{
			Context: "", // TODO
		})))
	} else {
		// Resuming a step
		messages = append(messages, state.Messages...)
		if len(state.Messages) < 32 {
			messages = append(messages, llm.NewUserMessage(continueStepPrompt))
		} else {
			messages = append(messages, llm.NewUserMessage(finishStepNowPrompt))
		}
	}

	logger.Info(
		"handling task",
		"status", state.Status,
		"truncated_description", TruncateText(task.Description(), 10),
		"truncated_prompt", TruncateText(task.Prompt(dive.TaskPromptOptions{}), 10),
		"message_count", len(messages),
	)

	// Run the LLM generation and any resulting tool calls
	response, updatedMessages, err := a.generate(
		ctx,
		messages,
		systemPrompt,
		task.Name(),
		state.Publisher,
	)
	if err != nil {
		return err
	}
	state.TrackResponse(response, updatedMessages)

	logger.Info("step updated",
		"status", state.Status,
		"status_description", state.StatusDescription(),
	)
	return nil
}

func (a *Agent) doSomeWork() {

	// Helper function to safely send events to the active task's publisher
	safePublish := func(event *events.Event) error {
		if a.activeTask.Publisher == nil {
			return nil
		}
		return a.activeTask.Publisher.Send(context.Background(), event)
	}

	// Activate the next task if there is one and we're idle
	if a.activeTask == nil && len(a.taskQueue) > 0 {
		// Pop and activate the first task in queue
		a.activeTask = a.taskQueue[0]
		a.taskQueue = a.taskQueue[1:]
		a.activeTask.Status = dive.TaskStatusActive
		if !a.activeTask.Paused {
			a.activeTask.Started = time.Now()
		} else {
			a.activeTask.Paused = false
		}
		safePublish(&events.Event{
			Type:      "task.activated",
			TaskName:  a.activeTask.Task.Name(),
			AgentName: a.name,
		})
		a.logger.Debug("task activated",
			"agent", a.name,
			"task", a.activeTask.Task.Name(),
			"description", a.activeTask.Task.Description(),
		)
	}

	if a.activeTask == nil {
		return // Nothing to do!
	}
	stepName := a.activeTask.Task.Name()

	// Make progress on the active task
	err := a.handleTask(context.Background(), a.activeTask)

	// An error deactivates the task and pushes an error event on the stream
	if err != nil {
		a.activeTask.Status = dive.TaskStatusError
		a.rememberTask(a.activeTask)
		safePublish(&events.Event{
			Type:      "task.error",
			TaskName:  stepName,
			AgentName: a.name,
			Error:     err.Error(),
		})
		a.logger.Error("task error",
			"agent", a.name,
			"task", stepName,
			"duration", time.Since(a.activeTask.Started).Seconds(),
			"error", err,
		)
		if a.activeTask.Publisher != nil {
			a.activeTask.Publisher.Close()
			a.activeTask.Publisher = nil
		}
		a.activeTask = nil
		return
	}

	// Handle task state transitions
	switch a.activeTask.Status {

	case dive.TaskStatusCompleted:
		a.rememberTask(a.activeTask)
		a.logger.Debug("task completed",
			"agent", a.name,
			"task", a.activeTask.Task.Name(),
			"duration", time.Since(a.activeTask.Started).Seconds(),
		)
		safePublish(&events.Event{
			Type:      "step.result",
			TaskName:  stepName,
			AgentName: a.name,
			Payload: &events.TaskResult{
				Task:    a.activeTask.Task,
				Usage:   a.activeTask.Usage,
				Content: a.activeTask.LastOutput(),
			},
		})
		if a.activeTask.Publisher != nil {
			a.activeTask.Publisher.Close()
			a.activeTask.Publisher = nil
		}
		a.activeTask = nil

	case dive.TaskStatusActive:
		a.logger.Debug("step remains active",
			"agent", a.name,
			"task", a.activeTask.Task.Name(),
			"status", a.activeTask.Status,
			"status_description", a.activeTask.StatusDescription,
			"duration", time.Since(a.activeTask.Started).Seconds(),
		)
		safePublish(&events.Event{
			Type:      "step.progress",
			TaskName:  stepName,
			AgentName: a.name,
		})

	case dive.TaskStatusPaused:
		// Set paused flag and return the task to the queue
		a.logger.Debug("step paused",
			"agent", a.name,
			"task", a.activeTask.Task.Name(),
		)
		safePublish(&events.Event{
			Type:      "step.paused",
			TaskName:  stepName,
			AgentName: a.name,
		})
		a.activeTask.Paused = true
		a.taskQueue = append(a.taskQueue, a.activeTask)
		a.activeTask = nil

	case dive.TaskStatusBlocked, dive.TaskStatusError, dive.TaskStatusInvalid:
		a.logger.Warn("task error",
			"agent", a.name,
			"task", a.activeTask.Task.Name(),
			"status", a.activeTask.Status,
			"status_description", a.activeTask.StatusDescription,
			"duration", time.Since(a.activeTask.Started).Seconds(),
		)
		safePublish(&events.Event{
			Type:      "task.error",
			TaskName:  stepName,
			AgentName: a.name,
			Error:     fmt.Sprintf("task status: %s", a.activeTask.Status),
		})
		if a.activeTask.Publisher != nil {
			a.activeTask.Publisher.Close()
			a.activeTask.Publisher = nil
		}
		a.activeTask = nil
	}
}

// Remember the last 10 tasks that were worked on, so that the agent can use
// them as context for future tasks.
func (a *Agent) rememberTask(task *taskState) {
	a.recentTasks = append(a.recentTasks, task)
	if len(a.recentTasks) > 10 {
		a.recentTasks = a.recentTasks[1:]
	}
}

// Returns a block of text that summarizes the most recent tasks worked on by
// the agent. The text is truncated if needed to avoid using a lot of tokens.
func (a *Agent) getTasksHistory() string {
	if len(a.recentTasks) == 0 {
		return ""
	}
	history := make([]string, len(a.recentTasks))
	for i, status := range a.recentTasks {
		title := status.Task.Name()
		if title == "" {
			title = status.Task.Description()
		}
		history[i] = fmt.Sprintf("- task: %q status: %q output: %q\n",
			TruncateText(title, 8),
			status.Status,
			TruncateText(replaceNewlines(status.LastOutput()), 8),
		)
	}
	result := strings.Join(history, "\n")
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

// Returns a user message that contains a summary of the most recent tasks
// worked on by the agent.
func (a *Agent) getTasksHistoryMessage() (*llm.Message, bool) {
	history := a.getTasksHistory()
	if history == "" {
		return nil, false
	}
	text := fmt.Sprintf("Recently completed tasks:\n\n%s", history)
	return llm.NewUserMessage(text), true
}

func (a *Agent) Fingerprint() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("agent: %s\n", a.name))
	sb.WriteString(fmt.Sprintf("description: %s\n", a.description))
	sb.WriteString(fmt.Sprintf("instructions: %s\n", a.instructions))
	sb.WriteString(fmt.Sprintf("accepted_events: %v\n", a.acceptedEvents))
	sb.WriteString(fmt.Sprintf("is_supervisor: %t\n", a.isSupervisor))
	sb.WriteString(fmt.Sprintf("subordinates: %v\n", a.subordinates))
	sb.WriteString(fmt.Sprintf("llm: %s\n", a.llm.Name()))
	hash := sha256.New()
	hash.Write([]byte(sb.String()))
	return hex.EncodeToString(hash.Sum(nil))
}

// if !a.disablePrefill {
// 	prefill := "<think>"
// 	if i >= a.generationLimit-1 {
// 		prefill += "I must respond with my final answer now."
// 	}
// 	generateOpts = append(generateOpts, llm.WithPrefill(prefill, "</think>"))
// }
