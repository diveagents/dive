package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
	"github.com/google/uuid"
)

// Environment is a container for running agents and workflow executions
type Environment struct {
	id              string
	name            string
	description     string
	agents          map[string]dive.Agent
	workflows       map[string]*workflow.Workflow
	triggers        []*Trigger
	executions      map[string]*Execution
	logger          slogger.Logger
	defaultWorkflow string
}

// EnvironmentOptions configures a new environment
type EnvironmentOptions struct {
	ID              string
	Name            string
	Description     string
	Agents          []dive.Agent
	Workflows       []*workflow.Workflow
	Triggers        []*Trigger
	Executions      []*Execution
	Logger          slogger.Logger
	DefaultWorkflow string
}

// New creates a new Environment instance
func New(opts EnvironmentOptions) (*Environment, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("environment name is required")
	}
	if opts.Logger == nil {
		opts.Logger = slogger.DefaultLogger
	}

	agents := make(map[string]dive.Agent, len(opts.Agents))
	for _, agent := range opts.Agents {
		if _, exists := agents[agent.Name()]; exists {
			return nil, fmt.Errorf("agent already registered: %s", agent.Name())
		}
		agents[agent.Name()] = agent
	}

	workflows := make(map[string]*workflow.Workflow, len(opts.Workflows))
	for _, workflow := range opts.Workflows {
		if _, exists := workflows[workflow.Name()]; exists {
			return nil, fmt.Errorf("workflow already registered: %s", workflow.Name())
		}
		workflows[workflow.Name()] = workflow
	}

	executions := make(map[string]*Execution, len(opts.Executions))
	for _, execution := range opts.Executions {
		executions[execution.ID()] = execution
	}

	if opts.DefaultWorkflow != "" {
		if _, exists := workflows[opts.DefaultWorkflow]; !exists {
			return nil, fmt.Errorf("default workflow not found: %s", opts.DefaultWorkflow)
		}
	}

	env := &Environment{
		id:              opts.ID,
		name:            opts.Name,
		description:     opts.Description,
		agents:          agents,
		workflows:       workflows,
		triggers:        opts.Triggers,
		executions:      executions,
		logger:          opts.Logger,
		defaultWorkflow: opts.DefaultWorkflow,
	}

	for _, trigger := range env.triggers {
		trigger.SetEnvironment(env)
	}

	for _, agent := range env.Agents() {
		agent.SetEnvironment(env)
	}

	for _, agent := range env.Agents() {
		if runnableAgent, ok := agent.(dive.RunnableAgent); ok {
			runnableAgent.Start(context.Background())
		}
	}

	return env, nil
}

func (e *Environment) ID() string {
	return e.id
}

func (e *Environment) Name() string {
	return e.name
}

func (e *Environment) Description() string {
	return e.description
}

func (e *Environment) Stop(ctx context.Context) {
	for _, agent := range e.Agents() {
		if runnableAgent, ok := agent.(dive.RunnableAgent); ok {
			runnableAgent.Stop(ctx)
		}
	}
	// TODO: stop executions
}

func (e *Environment) Agents() []dive.Agent {
	agents := make([]dive.Agent, 0, len(e.agents))
	for _, agent := range e.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (e *Environment) GetAgent(name string) (dive.Agent, error) {
	if agent, exists := e.agents[name]; exists {
		return agent, nil
	}
	return nil, fmt.Errorf("agent not found: %s", name)
}

func (e *Environment) RegisterAgent(agent dive.Agent) error {
	if _, exists := e.agents[agent.Name()]; exists {
		return fmt.Errorf("agent already present: %s", agent.Name())
	}
	e.agents[agent.Name()] = agent
	return nil
}

func (e *Environment) Workflows() []*workflow.Workflow {
	workflows := make([]*workflow.Workflow, 0, len(e.workflows))
	for _, workflow := range e.workflows {
		workflows = append(workflows, workflow)
	}
	return workflows
}

func (e *Environment) GetWorkflow(name string) (*workflow.Workflow, error) {
	if workflow, exists := e.workflows[name]; exists {
		return workflow, nil
	}
	return nil, fmt.Errorf("workflow not found: %s", name)
}

func (e *Environment) AddWorkflow(workflow *workflow.Workflow) error {
	if _, exists := e.workflows[workflow.Name()]; exists {
		return fmt.Errorf("workflow already present: %s", workflow.Name())
	}
	e.workflows[workflow.Name()] = workflow
	return nil
}

// StartWorkflow starts a new workflow execution
func (e *Environment) StartWorkflow(
	ctx context.Context,
	workflow *workflow.Workflow,
	inputs map[string]interface{},
) (*Execution, error) {
	if _, exists := e.workflows[workflow.Name()]; !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflow.Name())
	}

	// Build up the input variables with defaults and validation
	processedInputs := make(map[string]interface{})
	for name, input := range workflow.Inputs() {
		value, exists := inputs[name]
		if !exists {
			// If input doesn't exist, check if it has a default value
			if input.Default != nil {
				processedInputs[name] = input.Default
				continue
			}
			return nil, fmt.Errorf("required input %q not provided", name)
		}
		// Input exists, use the provided value
		processedInputs[name] = value
	}

	execution := NewExecution(ExecutionOptions{
		ID:          uuid.New().String(),
		Environment: e,
		Workflow:    workflow,
		Status:      StatusPending,
		StartTime:   time.Now(),
		Inputs:      processedInputs,
		Logger:      e.logger,
	})
	e.executions[execution.ID()] = execution

	go func() {
		if err := execution.Run(ctx); err != nil {
			e.logger.Error("workflow execution failed", "error", err)
			return
		}
	}()

	return execution, nil
}
