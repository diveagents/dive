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
	DocumentRepository document.Repository
}

// Agent is the standard implementation of the Agent interface.
type Agent struct {
	name               string
	description        string
	instructions       string
	llm                llm.LLM
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
	hooks              llm.Hooks
	logger             slogger.Logger
	toolIterationLimit int
	threads            map[string]*chatThread
	temperature        *float64
	presencePenalty    *float64
	frequencyPenalty   *float64
	reasoningFormat    string
	reasoningEffort    string
	dateAwareness      *bool
	environment        dive.Environment
	documentRepository document.Repository

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
		opts.Logger = slogger.DefaultLogger
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
		threads:            make(map[string]*chatThread),
		dateAwareness:      opts.DateAwareness,
		documentRepository: opts.DocumentRepository,
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

func (a *Agent) SetEnvironment(env dive.Environment) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.environment = env
}

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

func (a *Agent) Generate(ctx context.Context, message *llm.Message, opts ...dive.GenerateOption) (*llm.Response, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	var generateOptions dive.GenerateOptions
	generateOptions.Apply(opts)

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

func (a *Agent) Stream(ctx context.Context, message *llm.Message, opts ...dive.GenerateOption) (dive.Stream, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	var generateOptions dive.GenerateOptions
	generateOptions.Apply(opts)

	stream := dive.NewStream()

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

func (a *Agent) HandleEvent(ctx context.Context, event *dive.Event) error {
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

func (a *Agent) Work(ctx context.Context, task dive.Task, inputs map[string]any) (dive.Stream, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	// Stream to be returned to the caller so it can wait for results
	stream := dive.NewStream()

	message := messageWork{
		task:      task,
		inputs:    inputs,
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

			case *dive.Event:
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
		Inputs:    m.inputs,
	})
}

func (a *Agent) handleEvent(event *dive.Event) {
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
	var publisher dive.Publisher
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
	publisher dive.Publisher,
) (*llm.Response, []*llm.Message, error) {
	// Holds the most recent response from the LLM
	var response *llm.Response
	updatedMessages := make([]*llm.Message, len(messages))
	copy(updatedMessages, messages)

	// Helper function to safely send events to the publisher
	safePublish := func(event *dive.Event) error {
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
	for i := range generationLimit {
		generateOpts := []llm.Option{
			llm.WithSystemPrompt(systemPrompt),
			llm.WithCacheControl(a.cacheControl),
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
			iterator, err := streamingLLM.Stream(ctx, updatedMessages, generateOpts...)
			if err != nil {
				return nil, nil, err
			}
			for iterator.Next() {
				event := iterator.Event()
				if err := safePublish(&dive.Event{
					Type:    "llm.event",
					Origin:  a.eventOrigin(),
					Payload: event,
				}); err != nil {
					iterator.Close()
					return nil, nil, err
				}
				if event.Response != nil {
					currentResponse = event.Response
				}
			}
			iterator.Close()
			if err := iterator.Err(); err != nil {
				return nil, nil, err
			}
		} else {
			var err error
			currentResponse, err = a.llm.Generate(ctx, updatedMessages, generateOpts...)
			if err != nil {
				return nil, nil, err
			}
		}

		if currentResponse == nil {
			// This indicates a bug in the LLM provider implementation
			return nil, nil, errors.New("no final response from llm provider")
		}

		if err := safePublish(&dive.Event{
			Type:    "llm.response",
			Origin:  a.eventOrigin(),
			Payload: currentResponse,
		}); err != nil {
			return nil, nil, err
		}

		response = currentResponse
		responseMessage := response.Message()

		a.logger.Debug("llm response",
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
				return nil, nil, fmt.Errorf("tool call for unknown tool: %q", toolCall.Name)
			}
			a.logger.Debug("executing tool call",
				"tool_id", toolCall.ID,
				"tool_name", toolCall.Name,
				"tool_input", toolCall.Input)

			result, err := tool.Call(ctx, toolCall.Input)
			if err != nil {
				return nil, nil, fmt.Errorf("tool call error: %w", err)
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
		if a.logger != nil {
			var toolResultIDs []string
			for _, result := range toolResults {
				toolResultIDs = append(toolResultIDs, result.ID)
			}
		}

		// Add instructions to the message to not use any more tools if we have
		// only one generation left
		if i == generationLimit-2 {
			resultMessage.Content = append(resultMessage.Content, &llm.Content{
				Type: llm.ContentTypeText,
				Text: "Do not use any more tools. You must respond with your final answer now.",
			})
			a.logger.Debug("added tool use limit instruction",
				"agent", a.name,
				"step", stepName,
				"generation_number", i+1)
		}
		updatedMessages = append(updatedMessages, resultMessage)
	}

	return response, updatedMessages, nil
}

// func (a *Agent) getTaskDocumentsMessage(ctx context.Context, task dive.Task) (*llm.Message, error) {
// 	documents, err := a.loadTaskDocuments(ctx, task)
// 	if err != nil {
// 		return nil, err
// 	}
// 	if len(documents) == 0 {
// 		return nil, nil
// 	}
// 	var parts []string
// 	for _, doc := range documents {
// 		text := fmt.Sprintf("<document id=%q name=%q>\n%s\n</document>",
// 			doc.ID(), doc.Name(), doc.Content())
// 		parts = append(parts, text)
// 	}
// 	return llm.NewUserMessage(strings.Join(parts, "\n\n")), nil
// }

func (a *Agent) TeamOverview() string {
	// return a.environment.Team().Overview()
	return ""
}

func (a *Agent) handleTask(ctx context.Context, state *taskState) error {
	task := state.Task

	timeout := task.Timeout()
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
	logger.Info("handling task", "status", state.Status)

	systemPrompt, err := a.getSystemPromptForMode("task")
	if err != nil {
		return err
	}

	// documentsMessage, err := a.getTaskDocumentsMessage(ctx, task)
	// if err != nil {
	// 	return err
	// }

	var prompt string
	messages := []*llm.Message{}

	if len(state.Messages) == 0 {
		// Starting a step
		if recentTasksMessage, ok := a.getTasksHistoryMessage(); ok {
			messages = append(messages, recentTasksMessage)
		}
		// if documentsMessage != nil {
		// 	messages = append(messages, documentsMessage)
		// }
		var err error
		prompt, err = task.Prompt(dive.TaskPromptOptions{
			Context: "", // TODO
			Inputs:  state.Inputs,
		})
		if err != nil {
			return err
		}
		messages = append(messages, llm.NewUserMessage(prompt))
	} else {
		// Resuming a step
		messages = append(messages, state.Messages...)
		if len(messages) < 32 {
			messages = append(messages, llm.NewUserMessage(continueStepPrompt))
		} else {
			messages = append(messages, llm.NewUserMessage(finishStepNowPrompt))
		}
	}

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

	if state.Status == dive.TaskStatusCompleted {
		if err := a.saveOutput(ctx, task, state, logger); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) getDocumentRepository() (document.Repository, bool) {
	if a.documentRepository != nil {
		return a.documentRepository, true
	}
	if a.environment != nil {
		repo := a.environment.DocumentRepository()
		if repo != nil {
			return repo, true
		}
	}
	return nil, false
}

func (a *Agent) saveOutput(ctx context.Context, task dive.Task, state *taskState, logger slogger.Logger) error {
	output := task.Output()
	if output == nil {
		return nil
	}
	repo, ok := a.getDocumentRepository()
	if !ok {
		return fmt.Errorf("no document repository available")
	}
	if output.Document == "" {
		return nil
	}
	// Look up the path for the document with that name
	docMeta, ok := a.getDocumentMetadata(output.Document)
	if !ok {
		return fmt.Errorf("document not found: %q", output.Document)
	}
	doc, err := repo.GetDocument(ctx, docMeta.Path)
	if err != nil {
		return err
	}
	doc.SetContent(state.LastOutput())
	if err := repo.PutDocument(ctx, doc); err != nil {
		return err
	}
	logger.Info("saved output document", "document", output.Document)
	return nil
}

func (a *Agent) getDocumentMetadata(docName string) (*document.Metadata, bool) {
	if a.environment == nil {
		return nil, false
	}
	docMeta, ok := a.environment.KnownDocuments()[docName]
	if !ok {
		return nil, false
	}
	return docMeta, true
}

func (a *Agent) eventOrigin() dive.EventOrigin {
	var taskName string
	if a.activeTask != nil {
		taskName = a.activeTask.Task.Name()
	}
	var environmentName string
	if a.environment != nil {
		environmentName = a.environment.Name()
	}
	return dive.EventOrigin{
		AgentName:       a.name,
		TaskName:        taskName,
		EnvironmentName: environmentName,
	}
}

func (a *Agent) doSomeWork() {

	// Helper function to safely send events to the active task's publisher
	safePublish := func(event *dive.Event) error {
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
		safePublish(&dive.Event{
			Type:   "task.activated",
			Origin: a.eventOrigin(),
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
		safePublish(&dive.Event{
			Type:   "task.error",
			Origin: a.eventOrigin(),
			Error:  err,
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
		safePublish(&dive.Event{
			Type:   "task.result",
			Origin: a.eventOrigin(),
			Payload: &dive.TaskResult{
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
		safePublish(&dive.Event{
			Type:   "task.progress",
			Origin: a.eventOrigin(),
		})

	case dive.TaskStatusPaused:
		// Set paused flag and return the task to the queue
		a.logger.Debug("step paused",
			"agent", a.name,
			"task", a.activeTask.Task.Name(),
		)
		safePublish(&dive.Event{
			Type:   "task.paused",
			Origin: a.eventOrigin(),
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
		safePublish(&dive.Event{
			Type:   "task.error",
			Origin: a.eventOrigin(),
			Error:  fmt.Errorf("task status: %s", a.activeTask.Status),
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
