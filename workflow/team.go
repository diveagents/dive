package dive

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/stream"
	"github.com/getstingrai/dive/workflow"
)

var _ Team = &DiveTeam{}

// DiveTeam is the primary implementation of the Team interface. A Team consists
// of one or more Agents that work together to complete tasks.
type DiveTeam struct {
	name         string
	description  string
	agents       []Agent
	supervisors  []Agent
	running      bool
	initialSteps []*Step
	outputDir    string
	outputPlugin OutputPlugin
	documents    DocumentStore
	logLevel     string
	logger       slogger.Logger
	mutex        sync.Mutex
}

// TeamOptions are used to configure a new team.
type TeamOptions struct {
	Name         string
	Description  string
	Agents       []Agent
	Steps        []*Step
	Documents    DocumentStore
	LogLevel     string
	Logger       slogger.Logger
	OutputDir    string
	OutputPlugin OutputPlugin
}

// NewTeam creates a new team composed of the given agents.
func NewTeam(opts TeamOptions) (*DiveTeam, error) {
	if opts.Logger == nil {
		opts.Logger = DefaultLogger
	}
	if opts.OutputPlugin == nil {
		if opts.OutputDir == "" {
			opts.OutputPlugin = NewInMemoryOutputPlugin()
		} else {
			var err error
			opts.OutputPlugin, err = NewDiskOutputPlugin(opts.OutputDir)
			if err != nil {
				return nil, fmt.Errorf("failed to create disk output plugin: %w", err)
			}
		}
	}
	t := &DiveTeam{
		name:         opts.Name,
		description:  opts.Description,
		agents:       opts.Agents,
		initialSteps: opts.Steps,
		logLevel:     opts.LogLevel,
		logger:       opts.Logger,
		outputDir:    opts.OutputDir,
		outputPlugin: opts.OutputPlugin,
		documents:    opts.Documents,
	}
	for _, step := range opts.Steps {
		if err := step.Validate(); err != nil {
			return nil, err
		}
	}
	if len(t.agents) == 0 {
		return nil, fmt.Errorf("at least one agent is required")
	}
	for _, agent := range t.agents {
		if name := agent.Name(); name == "" {
			return nil, fmt.Errorf("agent has no name")
		}
		if teamAgent, ok := agent.(TeamAgent); ok {
			if err := teamAgent.Join(t); err != nil {
				return nil, err
			}
			if teamAgent.IsSupervisor() {
				t.supervisors = append(t.supervisors, agent)
			}
		}
	}
	if len(t.agents) > 1 && len(t.supervisors) == 0 {
		return nil, fmt.Errorf("at least one supervisor is required")
	}
	t.logger.Info("team created",
		"output_plugin", t.outputPlugin.Name())
	return t, nil
}

// Description of the team.
func (t *DiveTeam) Description() string {
	return t.description
}

// Agents returns a copy of the agents in the team.
func (t *DiveTeam) Agents() []Agent {
	// Make a copy to help ensure immutability on the set
	agents := make([]Agent, len(t.agents))
	copy(agents, t.agents)
	return agents
}

// Name returns the name of the team.
func (t *DiveTeam) Name() string {
	return t.name
}

// DocumentStore returns the document store for the team.
func (t *DiveTeam) DocumentStore() document.DocumentStore {
	return t.documents
}

// IsRunning returns true if the team is active.
func (t *DiveTeam) IsRunning() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	return t.running
}

// Start all agents belonging to the team.
func (t *DiveTeam) Start(ctx context.Context) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if err := t.start(ctx); err != nil {
		t.logger.Error("failed to start team", "error", err)
		return err
	}
	return nil
}

// Start all agents on the team. Call only when the team mutex is held.
func (t *DiveTeam) start(ctx context.Context) error {
	if len(t.agents) == 0 {
		return fmt.Errorf("no agents to start")
	}
	if t.running {
		return fmt.Errorf("team already running")
	}

	// Start all agents, but if any fail, stop them all before returning the error
	var startedAgents []RunnableAgent
	for _, agent := range t.agents {
		runnableAgent, ok := agent.(RunnableAgent)
		if !ok {
			continue
		}
		if err := runnableAgent.Start(ctx); err != nil {
			for _, startedAgent := range startedAgents {
				startedAgent.Stop(ctx)
			}
			return fmt.Errorf("failed to start agent %q: %w", agent.Name(), err)
		}
		startedAgents = append(startedAgents, runnableAgent)
	}

	t.logger.Debug("team started",
		"team_name", t.name,
		"team_description", t.description,
		"agent_count", len(t.agents),
		"agent_names", AgentNames(t.agents),
	)
	t.running = true
	return nil
}

// Stop all agents belonging to the team.
func (t *DiveTeam) Stop(ctx context.Context) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if !t.running {
		return fmt.Errorf("team not running")
	}
	t.running = false

	var lastErr error
	for _, agent := range t.agents {
		runnableAgent, ok := agent.(RunnableAgent)
		if !ok {
			continue
		}
		if err := runnableAgent.Stop(ctx); err != nil {
			lastErr = fmt.Errorf("failed to stop agent %s: %w", agent.Name(), err)
		}
	}

	t.logger.Debug("team stopped", "team_name", t.name)
	return lastErr
}

// Work on one or more tasks. The returned stream will deliver events and
// results to the caller as progress is made. This batch of work is considered
// independent of any other work the team may be doing. If the team has not yet
// started, it is automatically started.
func (t *DiveTeam) Work(ctx context.Context, steps ...*workflow.Step) (*stream.Stream, error) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	var todo []*workflow.Step

	// Automatically start as needed
	if !t.running {
		if err := t.start(ctx); err != nil {
			return nil, err
		}
	}
	// Any initial steps are enqueued first
	if len(t.initialSteps) > 0 {
		todo = append(todo, t.initialSteps...)
		t.initialSteps = nil
	}
	// Add any steps provided by the caller
	if len(steps) > 0 {
		todo = append(todo, steps...)
	}
	// Make sure we have something to do
	if len(todo) == 0 {
		return nil, fmt.Errorf("no steps to work on")
	}

	// Validate and index steps by name
	stepsByName := make(map[string]*Step, len(todo))
	for _, step := range todo {
		if err := step.Validate(); err != nil {
			return nil, err
		}
		name := step.Name()
		if stepsByName[name] != nil {
			return nil, fmt.Errorf("duplicate step name: %q", name)
		}
		stepsByName[name] = step
	}

	// Sort steps into execution order
	orderedNames, err := workflow.OrderSteps(todo)
	if err != nil {
		return nil, fmt.Errorf("failed to determine step execution order: %w", err)
	}
	var orderedSteps []*workflow.Step
	for _, stepName := range orderedNames {
		orderedSteps = append(orderedSteps, stepsByName[stepName])
	}

	// This stream will be used to deliver events and results to the caller
	stream := stream.New()

	// Run work and process events in a separate goroutine
	go t.workOnSteps(ctx, orderedSteps, stream)

	return stream, nil
}

func (t *DiveTeam) workOnSteps(ctx context.Context, steps []*workflow.Step, stream *stream.Stream) {
	publisher := stream.NewPublisher()
	defer publisher.Close()

	backgroundCtx := context.Background()

	t.logger.Debug("team work started",
		"team_name", t.name,
		"step_count", len(steps),
		"step_names", StepNames(steps),
	)

	totalUsage := llm.Usage{}

	// Work on tasks sequentially
	for _, step := range steps {

		// Determine which agent should take the task
		var agent Agent
		if step.AssignedAgent() != nil {
			agent = step.AssignedAgent()
		} else if len(t.supervisors) > 0 {
			agent = t.supervisors[0]
		} else {
			agent = t.agents[0]
		}

		// Capture the output of any dependencies and store on the task
		if dependencies := step.Dependencies(); len(dependencies) > 0 {
			var outputs []string
			for _, dep := range dependencies {
				depResult, err := t.outputPlugin.ReadOutput(ctx, dep, "")
				if err != nil {
					// This should never happen since the tasks were sorted into
					// execution order! If it does, it indicates a severe bug so
					// a panic is appropriate.
					panic(fmt.Sprintf("task execution failure: task %q dependency %q", step.Name(), dep))
				}
				outputs = append(outputs, fmt.Sprintf("<output task=%q>\n%s\n</output>", dep, depResult))
			}
			step.SetDependenciesOutput(strings.Join(outputs, "\n\n"))
		}

		// Has this work already been done?
		done, err := t.outputPlugin.OutputExists(ctx, step.Name(), "")
		if err != nil {
			t.logger.Error("failed to check if step output exists", "error", err)
		}
		if !done {
			// Work the step to completion
			result, err := t.workOnStep(ctx, step, agent, publisher)
			if err != nil {
				publisher.Send(backgroundCtx, &StreamEvent{
					Type:      "work.error",
					StepName:  step.Name(),
					AgentName: agent.Name(),
					Error:     fmt.Sprintf("work failed on step %q agent %q: %v", step.Name(), agent.Name(), err),
				})
				return
			}
			// Store the step results
			if err := t.outputPlugin.WriteOutput(ctx, step.Name(), "", result.Content); err != nil {
				publisher.Send(backgroundCtx, &stream.Event{
					Type:      "work.error",
					StepName:  step.Name(),
					AgentName: agent.Name(),
					Error:     fmt.Sprintf("failed to write output for step %q: %v", step.Name(), err),
				})
				return
			}
			totalUsage.InputTokens += result.Usage.InputTokens
			totalUsage.OutputTokens += result.Usage.OutputTokens
			totalUsage.CacheCreationInputTokens += result.Usage.CacheCreationInputTokens
			totalUsage.CacheReadInputTokens += result.Usage.CacheReadInputTokens
		} else {
			t.logger.Info("step output already exists - skipping", "step_name", step.Name())
		}
	}

	t.logger.Info("team work completed",
		"team_name", t.name,
		"total_input_tokens", totalUsage.InputTokens,
		"total_output_tokens", totalUsage.OutputTokens,
		"total_cache_creation_input_tokens", totalUsage.CacheCreationInputTokens,
		"total_cache_read_input_tokens", totalUsage.CacheReadInputTokens,
	)

	// Send a final event indicating all work is done. Use a clean context since
	// the provided context may have been canceled.
	publisher.Send(backgroundCtx, &StreamEvent{Type: "work.done"})
}

func (t *DiveTeam) workOnStep(ctx context.Context, step *Step, agent Agent, pub *StreamPublisher) (*StepResult, error) {
	workerAgent, ok := agent.(TeamAgent)
	if !ok {
		return nil, fmt.Errorf("agent %q does not accept steps", agent.Name())
	}

	stream, err := workerAgent.Work(ctx, step)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	logger := t.logger.With(
		"step_name", step.Name(),
		"agent_name", agent.Name(),
	)
	logger.Info("assigned step")

	// Heartbeats will indicate to the client that we're still going
	heartbeatTicker := time.NewTicker(time.Second * 3)
	defer heartbeatTicker.Stop()

	// Process all events from the task stream. Return when a task result is
	// found or the context is canceled. The worker handles the task timeouts.
	done := false
	for !done {
		select {
		// Forward all events via the publisher
		case event, ok := <-stream.Channel():
			if !ok {
				done = true
				continue
			}
			if err := pub.Send(ctx, event); err != nil {
				return nil, err // Canceled context probably
			}
			if event.Error != "" {
				logger.Error("task failed", "error", event.Error)
				return nil, errors.New(event.Error)
			}
			if event.StepResult != nil {
				logger.Info("step completed")
				return event.StepResult, nil
			}

		// Send heartbeats periodically
		case <-heartbeatTicker.C:
			pub.Send(ctx, &StreamEvent{
				Type:      "step.heartbeat",
				StepName:  step.Name(),
				AgentName: agent.Name(),
			})

		// Abort if the context is canceled
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Reaching this point may indicate a bug since a task result should have
	// been returned, even if it failed. Return an error in any case.
	return nil, fmt.Errorf("step %q did not return a result", step.Name())
}

// GetAgent returns the agent with the given name
func (t *DiveTeam) GetAgent(name string) (Agent, bool) {
	for _, agent := range t.agents {
		if agent.Name() == name {
			return agent, true
		}
	}
	return nil, false
}

// Overview returns a string representation of the team, which can be included
// in agent prompts to help them understand the team's capabilities.
func (t *DiveTeam) Overview() (string, error) {
	return workflow.ExecuteTemplate(teamPromptTemplate, t)
}

// HandleEvent passes an event to any agents that accept it.
func (t *DiveTeam) HandleEvent(ctx context.Context, event *Event) error {
	for _, agent := range t.agents {
		eventedAgent, ok := agent.(EventHandlerAgent)
		if !ok {
			continue
		}
		acceptedEvents := eventedAgent.AcceptedEvents()
		if !sliceContains(acceptedEvents, "*") && !sliceContains(acceptedEvents, event.Name) {
			continue
		}
		if err := eventedAgent.HandleEvent(ctx, event); err != nil {
			return err
		}
		t.logger.Debug("passed event to agent",
			"event_name", event.Name,
			"agent_name", agent.Name(),
		)
	}
	return nil
}

func AgentNames(agents []Agent) []string {
	var agentNames []string
	for _, agent := range agents {
		agentNames = append(agentNames, agent.Name())
	}
	return agentNames
}

func StepNames(steps []*Step) []string {
	var stepNames []string
	for _, step := range steps {
		stepNames = append(stepNames, step.Name())
	}
	return stepNames
}
