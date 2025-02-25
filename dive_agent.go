package dive

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/logger"
	"github.com/getstingrai/dive/prompt"
)

var _ Agent = &DiveAgent{}

type Document struct {
	Name    string
	Content string
	Format  OutputFormat
}

type AgentOptions struct {
	Name           string
	Role           Role
	LLM            llm.LLM
	Tools          []llm.Tool
	MaxActiveTasks int
	TickFrequency  time.Duration
	CacheControl   string
	LogLevel       string
	Hooks          llm.Hooks
	Logger         logger.Logger
}

// DiveAgent implements the Agent interface.
type DiveAgent struct {
	name           string
	role           Role
	llm            llm.LLM
	team           Team
	running        bool
	tools          []llm.Tool
	toolsByName    map[string]llm.Tool
	isSupervisor   bool
	isWorker       bool
	maxActiveTasks int
	tickFrequency  time.Duration
	cacheControl   string
	taskQueue      []*taskState
	activeTask     *taskState
	workspace      []*Document
	ticker         *time.Ticker
	recentTasks    []*taskState
	logLevel       string
	hooks          llm.Hooks
	logger         logger.Logger

	// Consolidate all message types into a single channel
	mailbox chan interface{}

	mutex sync.Mutex
	wg    sync.WaitGroup
}

// NewAgent returns a new Agent configured with the given options.
func NewAgent(opts AgentOptions) *DiveAgent {
	if opts.MaxActiveTasks == 0 {
		opts.MaxActiveTasks = 1
	}
	if opts.TickFrequency == 0 {
		opts.TickFrequency = time.Millisecond * 250
	}
	a := &DiveAgent{
		name:           opts.Name,
		role:           opts.Role,
		llm:            opts.LLM,
		maxActiveTasks: opts.MaxActiveTasks,
		tickFrequency:  opts.TickFrequency,
		cacheControl:   opts.CacheControl,
		mailbox:        make(chan interface{}, 64),
		toolsByName:    make(map[string]llm.Tool),
		logLevel:       strings.ToLower(opts.LogLevel),
		hooks:          opts.Hooks,
		logger:         opts.Logger,
	}
	var tools []llm.Tool
	if len(opts.Tools) > 0 {
		tools = make([]llm.Tool, len(opts.Tools))
		copy(tools, opts.Tools)
	}
	if opts.Role.IsSupervisor {
		tools = append(tools, NewAssignWorkTool(a))
	}
	a.tools = tools
	for _, tool := range tools {
		a.toolsByName[tool.Definition().Name] = tool
	}
	if a.logger == nil {
		a.logger = logger.NewSlogLogger(nil)
	}
	return a
}

func (a *DiveAgent) Name() string {
	return a.name
}

func (a *DiveAgent) Role() Role {
	return a.role
}

func (a *DiveAgent) Join(team Team) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.running {
		return fmt.Errorf("agent is already running")
	}
	if a.team != nil {
		return fmt.Errorf("agent is already a member of a team")
	}
	a.team = team
	return nil
}

func (a *DiveAgent) Team() Team {
	return a.team
}

func (a *DiveAgent) Log(msg string, keysAndValues ...any) {
	switch a.logLevel {
	case "debug":
		a.logger.Debug(msg, keysAndValues...)
	case "info":
		a.logger.Info(msg, keysAndValues...)
	case "warn":
		a.logger.Warn(msg, keysAndValues...)
	case "error":
		a.logger.Error(msg, keysAndValues...)
	}
}

func (a *DiveAgent) Chat(ctx context.Context, message *llm.Message) (*llm.Response, error) {
	result := make(chan *llm.Response, 1)
	errChan := make(chan error, 1)

	chatMessage := messageChat{
		ctx:     ctx,
		message: message,
		result:  result,
		err:     errChan,
	}

	select {
	case a.mailbox <- chatMessage:
		select {
		case resp := <-result:
			return resp, nil
		case err := <-errChan:
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *DiveAgent) ChatStream(ctx context.Context, message *llm.Message) (llm.Stream, error) {
	return nil, errors.New("not yet implemented")
}

func (a *DiveAgent) Event(ctx context.Context, event *Event) error {
	if !a.IsRunning() {
		return fmt.Errorf("agent is not running")
	}

	message := messageEvent{event: event}

	select {
	case a.mailbox <- message:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *DiveAgent) Work(ctx context.Context, task *Task) (*Promise, error) {
	if !a.IsRunning() {
		return nil, fmt.Errorf("agent is not running")
	}

	promise := &Promise{
		task: task,
		ch:   make(chan *TaskResult, 1),
	}

	message := messageWork{
		task:    task,
		promise: promise,
	}

	select {
	case a.mailbox <- message:
		return promise, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a *DiveAgent) Start(ctx context.Context) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if a.running {
		return fmt.Errorf("agent is already running")
	}

	a.running = true
	a.wg.Add(1)
	go a.run()
	a.Log("agent started", "name", a.name)
	return nil
}

func (a *DiveAgent) Stop(ctx context.Context) error {
	a.mutex.Lock()
	defer func() {
		a.running = false
		a.mutex.Unlock()
		a.Log("agent stopped", "name", a.name)
	}()

	if !a.running {
		return fmt.Errorf("agent is not running")
	}
	done := make(chan error)

	a.mailbox <- messageStop{
		ctx:  ctx,
		done: done,
	}
	close(a.mailbox)

	select {
	case err := <-done:
		a.wg.Wait()
		return err
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for agent to stop: %w", ctx.Err())
	}
}

func (a *DiveAgent) IsRunning() bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.running
}

func (a *DiveAgent) run() error {
	defer a.wg.Done()

	a.ticker = time.NewTicker(a.tickFrequency)
	defer a.ticker.Stop()

	for {
		select {
		case <-a.ticker.C:
			a.doSomeWork()
		case msg := <-a.mailbox:
			switch m := msg.(type) {
			case messageWork:
				a.handleWorkMessage(m)
			case messageChat:
				a.handleChatMessage(m)
			case messageEvent:
				a.handleEventMessage(m.event)
			case messageStop:
				return a.handleStopMessage(m)
			}
		}
	}
}

func (a *DiveAgent) handleWorkMessage(m messageWork) {
	a.taskQueue = append(a.taskQueue, &taskState{
		Task:    m.task,
		Promise: m.promise,
		Status:  TaskStatusQueued,
	})
}

func (a *DiveAgent) handleEventMessage(event *Event) {
	fmt.Printf("event: %+v\n", event)
}

func (a *DiveAgent) handleChatMessage(m messageChat) {
	task := &Task{
		name:        fmt.Sprintf("chat-%d", time.Now().UnixNano()),
		kind:        "chat",
		description: fmt.Sprintf("Generate a response to user message: %q", m.message.Text()),
		timeout:     time.Minute * 1,
	}
	// Enqueue a corresponding task state
	a.taskQueue = append(a.taskQueue, &taskState{
		Task:         task,
		Promise:      &Promise{ch: make(chan *TaskResult, 1)},
		Status:       TaskStatusQueued,
		Messages:     []*llm.Message{m.message},
		ChanResponse: m.result,
		ChanError:    m.err,
	})
}

func (a *DiveAgent) handleStopMessage(msg messageStop) error {
	msg.done <- nil
	return nil
}

func (a *DiveAgent) getSystemPrompt() (string, error) {
	data := NewAgentTemplateData(a)
	return executeTemplate(agentSystemPromptTemplate, data)
}

func (a *DiveAgent) handleTask(state *taskState) error {
	task := state.Task
	timeout := task.Timeout()
	if timeout == 0 {
		timeout = time.Minute * 3
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	systemPrompt, err := a.getSystemPrompt()
	if err != nil {
		return err
	}

	messages := []*llm.Message{}

	if len(state.Messages) == 0 {
		if len(a.recentTasks) > 0 {
			messages = append(messages, llm.NewUserMessage(
				fmt.Sprintf("For reference, here is an overview of other recent tasks we completed:\n\n%s", a.getRecentTasksHistory())))
		}
		messages = append(messages, llm.NewUserMessage(task.PromptText()))
	} else {
		messages = append(messages, state.Messages...)
		messages = append(messages, llm.NewUserMessage("Continue working on the task."))
	}

	a.Log("handle task",
		"name", task.Name(),
		"status", state.Status,
		"description", task.Description(),
		"prompt_text", task.PromptText(),
	)

	var response *llm.Response
	generationLimit := 8

	for i := 0; i < generationLimit; i++ {
		p, err := prompt.New(
			prompt.WithSystemMessage(systemPrompt),
			prompt.WithMessage(messages...),
			prompt.WithMessage(llm.NewAssistantMessage("<think>")),
		).Build()
		if err != nil {
			return err
		}
		generateOpts := []llm.Option{
			llm.WithSystemPrompt(p.System),
			llm.WithCacheControl(a.cacheControl),
			llm.WithLogLevel(a.logLevel),
		}
		if i == 0 || i < generationLimit-1 {
			generateOpts = append(generateOpts, llm.WithTools(a.tools...))
		}
		if a.hooks != nil {
			generateOpts = append(generateOpts, llm.WithHooks(a.hooks))
		}
		if a.logger != nil {
			generateOpts = append(generateOpts, llm.WithLogger(a.logger))
		}
		response, err = a.llm.Generate(ctx, p.Messages, generateOpts...)
		if err != nil {
			return err
		}

		// Mutate first text message response to include opening <think> tag
		responseMessage := response.Message()
		if len(responseMessage.Content) > 0 {
			for _, content := range responseMessage.Content {
				// Only insert the opening <think> if we see the closing </think>
				if content.Type == llm.ContentTypeText && strings.Contains(content.Text, "</think>") {
					content.Text = "<think>" + content.Text
					break
				}
			}
		}

		messages = append(messages, responseMessage)
		if len(response.ToolCalls()) > 0 {
			var toolResults []*llm.ToolResult
			for _, toolCall := range response.ToolCalls() {
				tool, ok := a.toolsByName[toolCall.Name]
				if !ok {
					return fmt.Errorf("tool not found: %s", toolCall.Name)
				} else {
					result, err := tool.Call(ctx, toolCall.Input)
					if err != nil {
						return fmt.Errorf("tool error: %w", err)
					} else {
						toolResults = append(toolResults, &llm.ToolResult{
							ID:     toolCall.ID,
							Name:   toolCall.Name,
							Result: result,
						})
					}
				}
			}
			if len(toolResults) > 0 {
				messages = append(messages, llm.NewToolResultMessage(toolResults))
			}
		} else {
			break
		}
	}

	state.Messages = messages
	sr := ParseStructuredResponse(response.Message().Text())
	if state.Output == "" {
		state.Output = sr.Text
	} else {
		state.Output = fmt.Sprintf("%s\n\n%s", state.Output, sr.Text)
	}
	state.Reasoning = sr.Thinking
	state.StatusDescription = sr.StatusDescription
	state.Status = sr.Status()
	return nil
}

func (a *DiveAgent) doSomeWork() {

	// Activate the next task if there is one and we're idle
	if a.activeTask == nil && len(a.taskQueue) > 0 {
		// Pop and activate the first task in queue
		a.activeTask = a.taskQueue[0]
		a.taskQueue = a.taskQueue[1:]
		a.activeTask.Status = TaskStatusActive
		if !a.activeTask.Suspended {
			a.activeTask.Started = time.Now()
		}
		a.activeTask.Suspended = false
		a.logger.Info("task activated", "name", a.activeTask.Task.Name())
	}

	if a.activeTask == nil {
		return
	}

	// Make progress on the active task
	err := a.handleTask(a.activeTask)

	// An error deactivates the task and we send the error via the promise
	if err != nil {
		duration := time.Since(a.activeTask.Started)
		a.logger.Error("task error",
			"name", a.activeTask.Task.Name(),
			"duration", duration.Seconds(),
			"error", err,
		)

		a.activeTask.Status = TaskStatusError
		a.rememberTask(a.activeTask)
		a.activeTask.Promise.ch <- NewTaskResultError(a.activeTask.Task, err)
		a.activeTask = nil
		return
	}

	switch a.activeTask.Status {
	case TaskStatusCompleted:
		duration := time.Since(a.activeTask.Started)
		a.logger.Info("task completed",
			"name", a.activeTask.Task.Name(),
			"duration", duration.Seconds(),
		)
		a.activeTask.Status = TaskStatusCompleted
		a.rememberTask(a.activeTask)
		if a.activeTask.Task.Kind() == "chat" {
			a.activeTask.ChanResponse <- llm.NewResponse(llm.ResponseOptions{
				Role:    llm.Assistant,
				Message: llm.NewAssistantMessage(a.activeTask.Output),
			})
		}
		if a.activeTask.Promise != nil {
			a.activeTask.Promise.ch <- &TaskResult{
				Task:    a.activeTask.Task,
				Content: a.activeTask.Output,
			}
		}
		a.activeTask = nil

	case TaskStatusPaused:
		a.logger.Info("task paused", "name", a.activeTask.Task.Name())
		a.activeTask.Suspended = true
		a.taskQueue = append(a.taskQueue, a.activeTask)
		a.activeTask = nil

	case TaskStatusBlocked, TaskStatusError, TaskStatusInvalid:
		a.logger.Info("task error",
			"name", a.activeTask.Task.Name(),
			"status", a.activeTask.Status,
			"status_description", a.activeTask.StatusDescription,
			"duration", time.Since(a.activeTask.Started).Seconds(),
		)
		if a.activeTask.Task.Kind() == "chat" && a.activeTask.ChanError != nil {
			a.activeTask.ChanError <- fmt.Errorf("task error: %s", a.activeTask.Status)
		}
		if a.activeTask.Promise != nil {
			a.activeTask.Promise.ch <- NewTaskResultError(a.activeTask.Task, fmt.Errorf("task error"))
		}
		a.activeTask = nil

	case TaskStatusActive:
		a.logger.Info("task remains active",
			"name", a.activeTask.Task.Name(),
			"status", a.activeTask.Status,
			"status_description", a.activeTask.StatusDescription,
			"duration", time.Since(a.activeTask.Started).Seconds(),
		)
	}
}

func (a *DiveAgent) rememberTask(task *taskState) {
	// Remember the last 10 tasks that were worked on, so that the agent can
	// use them as context for future tasks.
	a.recentTasks = append(a.recentTasks, task)
	if len(a.recentTasks) > 10 {
		a.recentTasks = a.recentTasks[1:]
	}
}

func (a *DiveAgent) getRecentTasksHistory() string {
	var history []string
	for _, status := range a.recentTasks {
		title := status.Task.Name()
		if title == "" {
			title = status.Task.Description()
		}
		title = TruncateText(title, 8)
		output := replaceNewlines(status.Output)
		history = append(history, fmt.Sprintf("- task: %q status: %q output: %q\n",
			title, status.Status, TruncateText(output, 8),
		))
	}
	result := strings.Join(history, "\n")
	if len(result) > 200 {
		result = result[:200]
	}
	return result
}

func (a *DiveAgent) getWorkspaceState() string {
	var blobs []string
	for _, doc := range a.workspace {
		blobs = append(blobs, fmt.Sprintf("<document name=%q>\n%s\n</document>", doc.Name, doc.Content))
	}
	return strings.Join(blobs, "\n\n")
}

type AgentTemplateData struct {
	Name      string
	Role      string
	Team      *Team
	IsManager bool
	IsWorker  bool
}

func TruncateText(text string, maxWords int) string {
	// Split into lines while preserving newlines
	lines := strings.Split(text, "\n")
	wordCount := 0
	var result []string
	// Process each line
	for _, line := range lines {
		words := strings.Fields(line)
		// If we haven't reached maxWords, add words from this line
		if wordCount < maxWords {
			remaining := maxWords - wordCount
			if len(words) <= remaining {
				// Add entire line if it fits
				if len(words) > 0 {
					result = append(result, line)
				} else {
					// Preserve empty lines
					result = append(result, "")
				}
				wordCount += len(words)
			} else {
				// Add partial line up to remaining words
				result = append(result, strings.Join(words[:remaining], " "))
				wordCount = maxWords
			}
		}
	}
	truncated := strings.Join(result, "\n")
	if wordCount >= maxWords {
		truncated += "..."
	}
	return truncated
}

var newlinesRegex = regexp.MustCompile(`\n+`)

func replaceNewlines(text string) string {
	return newlinesRegex.ReplaceAllString(text, "<br>")
}

type agentTemplateData struct {
	*DiveAgent
	DelegateTargets []Agent
}

func NewAgentTemplateData(agent *DiveAgent) *agentTemplateData {
	var delegateTargets []Agent
	if agent.role.IsSupervisor {
		if agent.role.Subordinates == nil {
			if agent.team != nil {
				// Unspecified means we can delegate to all non-supervisors
				for _, a := range agent.team.Agents() {
					if !a.Role().IsSupervisor {
						delegateTargets = append(delegateTargets, a)
					}
				}
			}
		} else if agent.team != nil {
			for _, name := range agent.role.Subordinates {
				other, found := agent.team.GetAgent(name)
				if found {
					delegateTargets = append(delegateTargets, other)
				}
			}
		}
	}
	return &agentTemplateData{
		DiveAgent:       agent,
		DelegateTargets: delegateTargets,
	}
}
