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

// Environment is the default implementation of Environment
type Environment struct {
	id          string
	name        string
	description string
	agents      map[string]dive.Agent
	workflows   map[string]*workflow.Workflow
	triggers    []*Trigger
	executions  map[string]*Execution
	logger      slogger.Logger
}

// EnvironmentOptions configures a new environment
type EnvironmentOptions struct {
	ID          string
	Name        string
	Description string
	Agents      []dive.Agent
	Workflows   []*workflow.Workflow
	Triggers    []*Trigger
	Executions  []*Execution
	Logger      slogger.Logger
}

// New creates a new Environment instance
func New(opts EnvironmentOptions) (*Environment, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("name is required")
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
	triggers := make([]*Trigger, len(opts.Triggers))
	for i, trigger := range opts.Triggers {
		triggers[i] = trigger
	}
	executions := make(map[string]*Execution, len(opts.Executions))
	for _, execution := range opts.Executions {
		executions[execution.ID] = execution
	}
	return &Environment{
		id:          opts.ID,
		name:        opts.Name,
		description: opts.Description,
		agents:      agents,
		workflows:   workflows,
		triggers:    triggers,
		executions:  executions,
		logger:      opts.Logger,
	}, nil
}

func (e *Environment) Name() string {
	return e.name
}

func (e *Environment) Description() string {
	return e.description
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

func (e *Environment) AddAgent(agent dive.Agent) error {
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

func (e *Environment) StartWorkflow(ctx context.Context, workflow *workflow.Workflow) (*Execution, error) {
	if _, exists := e.workflows[workflow.Name()]; !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflow.Name())
	}
	execution := workflow.NewExecution(workflow.ExecutionOptions{
		ID:        uuid.New().String(),
		Workflow:  workflow,
		Status:    workflow.StatusRunning,
		StartTime: time.Now(),
		EndTime:   time.Time{},
	})
	e.executions[execution.ID] = execution
	return execution, nil
}
