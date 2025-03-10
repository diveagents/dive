package environment

import (
	"context"
	"fmt"
	"sync"

	"github.com/getstingrai/dive"
)

// StandardEnvironment is the default implementation of Environment
type StandardEnvironment struct {
	name        string
	description string
	agents      map[string]dive.Agent
	workflows   map[string]dive.Workflow
	mutex       sync.RWMutex
}

// NewEnvironment creates a new StandardEnvironment instance
func NewEnvironment(name string, description string) *StandardEnvironment {
	return &StandardEnvironment{
		name:        name,
		description: description,
		agents:      make(map[string]dive.Agent),
		workflows:   make(map[string]dive.Workflow),
	}
}

func (e *StandardEnvironment) Name() string {
	return e.name
}

func (e *StandardEnvironment) Description() string {
	return e.description
}

func (e *StandardEnvironment) Agents() []dive.Agent {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	agents := make([]dive.Agent, 0, len(e.agents))
	for _, agent := range e.agents {
		agents = append(agents, agent)
	}
	return agents
}

func (e *StandardEnvironment) GetAgent(name string) (dive.Agent, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if agent, exists := e.agents[name]; exists {
		return agent, nil
	}
	return nil, fmt.Errorf("agent not found: %s", name)
}

func (e *StandardEnvironment) RegisterAgent(agent dive.Agent) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, exists := e.agents[agent.Name()]; exists {
		return fmt.Errorf("agent already registered: %s", agent.Name())
	}

	e.agents[agent.Name()] = agent
	return nil
}

func (e *StandardEnvironment) Workflows() []dive.Workflow {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	workflows := make([]dive.Workflow, 0, len(e.workflows))
	for _, workflow := range e.workflows {
		workflows = append(workflows, workflow)
	}
	return workflows
}

func (e *StandardEnvironment) GetWorkflow(id string) (dive.Workflow, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if workflow, exists := e.workflows[id]; exists {
		return workflow, nil
	}
	return nil, fmt.Errorf("workflow not found: %s", id)
}

func (e *StandardEnvironment) StartWorkflow(ctx context.Context, workflow dive.Workflow) error {
	if err := workflow.Validate(); err != nil {
		return fmt.Errorf("invalid workflow: %w", err)
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.workflows[workflow.Name()] = workflow
	return nil
}

func (e *StandardEnvironment) StopWorkflow(ctx context.Context, id string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, exists := e.workflows[id]; !exists {
		return fmt.Errorf("workflow not found: %s", id)
	}

	delete(e.workflows, id)
	return nil
}
