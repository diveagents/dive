package environment

import (
	"context"
	"fmt"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/mcp"
	"github.com/diveagents/dive/slogger"
	"github.com/diveagents/dive/workflow"
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
	documentRepo    dive.DocumentRepository
	threadRepo      dive.ThreadRepository
	actions         map[string]Action
	started         bool
	confirmer       dive.Confirmer
	mcpManager      *mcp.Manager
	mcpServers      []*mcp.ServerConfig
	formatter       WorkflowFormatter
}

// Options are used to configure an Environment.
type Options struct {
	ID                 string
	Name               string
	Description        string
	Agents             []dive.Agent
	Workflows          []*workflow.Workflow
	Triggers           []*Trigger
	Executions         []*Execution
	Logger             slogger.Logger
	DefaultWorkflow    string
	DocumentRepository dive.DocumentRepository
	ThreadRepository   dive.ThreadRepository
	Actions            []Action
	AutoStart          bool
	Confirmer          dive.Confirmer
	MCPServers         []*mcp.ServerConfig
	MCPManager         *mcp.Manager
	Formatter          WorkflowFormatter
}

// New returns a new Environment configured with the given options.
func New(opts Options) (*Environment, error) {
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

	actions := make(map[string]Action, len(opts.Actions))

	// Register document actions if we have a document repository
	if opts.DocumentRepository != nil {
		writeAction := NewDocumentWriteAction(opts.DocumentRepository)
		readAction := NewDocumentReadAction(opts.DocumentRepository)
		actions[writeAction.Name()] = writeAction
		actions[readAction.Name()] = readAction
	}
	for _, action := range actionsRegistry {
		actions[action.Name()] = action
	}
	for _, action := range opts.Actions {
		actions[action.Name()] = action
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
		documentRepo:    opts.DocumentRepository,
		threadRepo:      opts.ThreadRepository,
		actions:         actions,
		mcpManager:      opts.MCPManager,
		mcpServers:      opts.MCPServers,
		formatter:       opts.Formatter,
	}
	for _, trigger := range env.triggers {
		trigger.SetEnvironment(env)
	}
	for _, agent := range env.Agents() {
		agent.SetEnvironment(env)
	}

	if opts.AutoStart {
		if err := env.Start(context.Background()); err != nil {
			return nil, fmt.Errorf("failed to start environment: %w", err)
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

func (e *Environment) DefaultAgent() (dive.Agent, bool) {
	if len(e.agents) == 0 {
		return nil, false
	}

	// If there is one agent, that is the default
	if len(e.agents) == 1 {
		for _, agent := range e.agents {
			return agent, true
		}
	}

	// If there are 2+ agents, pick the first supervisor
	for _, agent := range e.agents {
		if agent.IsSupervisor() {
			return agent, true
		}
	}

	// If no supervisor found, return the first agent
	for _, agent := range e.agents {
		return agent, true
	}

	return nil, false
}

func (e *Environment) Formatter() WorkflowFormatter {
	return e.formatter
}

func (e *Environment) DocumentRepository() dive.DocumentRepository {
	return e.documentRepo
}

func (e *Environment) ThreadRepository() dive.ThreadRepository {
	return e.threadRepo
}

func (e *Environment) Confirmer() dive.Confirmer {
	return e.confirmer
}

func (e *Environment) Start(ctx context.Context) error {
	if e.started {
		return fmt.Errorf("environment already started")
	}

	if e.mcpManager != nil {
		if err := e.mcpManager.InitializeServers(ctx, e.mcpServers); err != nil {
			e.logger.Error("failed to initialize MCP servers", "error", err)
			return err
		}
	}

	e.started = true
	return nil
}

func (e *Environment) Stop(ctx context.Context) error {
	if !e.started {
		return fmt.Errorf("environment not started")
	}

	if e.mcpManager != nil {
		if err := e.mcpManager.Close(); err != nil {
			e.logger.Error("failed to close MCP connections", "error", err)
			return err
		}
	}

	// TODO: stop executions?
	e.started = false
	return nil
}

func (e *Environment) IsRunning() bool {
	return e.started
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

// GetAction returns an action by name
func (e *Environment) GetAction(name string) (Action, bool) {
	action, ok := e.actions[name]
	return action, ok
}

// GetMCPTools returns all MCP tools from all connected servers
func (e *Environment) GetMCPTools() map[string]dive.Tool {
	if e.mcpManager == nil {
		return make(map[string]dive.Tool)
	}
	return e.mcpManager.GetAllTools()
}

// GetMCPToolsByServer returns MCP tools from a specific server
func (e *Environment) GetMCPToolsByServer(serverName string) []dive.Tool {
	if e.mcpManager == nil {
		return nil
	}
	return e.mcpManager.GetToolsByServer(serverName)
}

// GetMCPTool returns a specific MCP tool by name (with server prefix)
func (e *Environment) GetMCPTool(toolKey string) dive.Tool {
	if e.mcpManager == nil {
		return nil
	}
	return e.mcpManager.GetTool(toolKey)
}

// GetMCPServerStatus returns the connection status of all MCP servers
func (e *Environment) GetMCPServerStatus() map[string]bool {
	if e.mcpManager == nil {
		return make(map[string]bool)
	}
	return e.mcpManager.GetServerStatus()
}
