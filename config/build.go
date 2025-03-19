package config

import (
	"fmt"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/environment"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/providers/groq"
	"github.com/getstingrai/dive/providers/openai"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
)

type BuildOptions struct {
	Variables  map[string]interface{}
	Tools      map[string]llm.Tool
	Logger     slogger.Logger
	LogLevel   string
	OutputDir  string
	Repository document.Repository
}

type BuildOption func(*BuildOptions)

func WithVariables(vars map[string]interface{}) BuildOption {
	return func(opts *BuildOptions) {
		opts.Variables = vars
	}
}

func WithTools(tools map[string]llm.Tool) BuildOption {
	return func(opts *BuildOptions) {
		opts.Tools = tools
	}
}

func WithLogger(logger slogger.Logger) BuildOption {
	return func(opts *BuildOptions) {
		opts.Logger = logger
	}
}

func WithOutputDir(dir string) BuildOption {
	return func(opts *BuildOptions) {
		opts.OutputDir = dir
	}
}

func WithRepository(repo document.Repository) BuildOption {
	return func(opts *BuildOptions) {
		opts.Repository = repo
	}
}

func buildAgent(agentDef AgentConfig, globalConfig Config, toolsMap map[string]llm.Tool, logger slogger.Logger) (dive.Agent, error) {
	provider := agentDef.Provider
	if provider == "" {
		provider = globalConfig.DefaultProvider
		if provider == "" {
			provider = "anthropic"
		}
	}

	model := agentDef.Model
	if model == "" {
		model = globalConfig.DefaultModel
	}

	var llmProvider llm.LLM
	switch provider {
	case "anthropic":
		opts := []anthropic.Option{}
		if model != "" {
			opts = append(opts, anthropic.WithModel(model))
		}
		llmProvider = anthropic.New(opts...)

	case "openai":
		opts := []openai.Option{}
		if model != "" {
			opts = append(opts, openai.WithModel(model))
		}
		llmProvider = openai.New(opts...)

	case "groq":
		opts := []groq.Option{}
		if model != "" {
			opts = append(opts, groq.WithModel(model))
		}
		llmProvider = groq.New(opts...)

	default:
		return nil, fmt.Errorf("unsupported provider: %q", provider)
	}

	var agentTools []llm.Tool
	for _, toolName := range agentDef.Tools {
		tool, ok := toolsMap[toolName]
		if !ok {
			return nil, fmt.Errorf("tool %q not found or not enabled", toolName)
		}
		agentTools = append(agentTools, tool)
	}

	var chatTimeout time.Duration
	if agentDef.ChatTimeout != "" {
		var err error
		chatTimeout, err = time.ParseDuration(agentDef.ChatTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid chat timeout: %w", err)
		}
	}

	cacheControl := agentDef.CacheControl
	if cacheControl == "" {
		cacheControl = globalConfig.CacheControl
	}

	logLevel := agentDef.LogLevel
	if logLevel == "" {
		logLevel = globalConfig.LogLevel
	}

	agent := agent.NewAgent(agent.AgentOptions{
		Name:               agentDef.Name,
		Description:        agentDef.Description,
		Instructions:       agentDef.Instructions,
		IsSupervisor:       agentDef.IsSupervisor,
		Subordinates:       agentDef.Subordinates,
		AcceptedEvents:     agentDef.AcceptedEvents,
		LLM:                llmProvider,
		Tools:              agentTools,
		ChatTimeout:        chatTimeout,
		CacheControl:       cacheControl,
		LogLevel:           logLevel,
		Logger:             logger,
		ToolIterationLimit: agentDef.ToolIterationLimit,
	})
	return agent, nil
}

func buildTask(taskDef Task, agents []dive.Agent) (*workflow.Task, error) {
	var timeout time.Duration
	if taskDef.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(taskDef.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
	}

	// Find assigned agent if specified
	var assignedAgent dive.Agent
	if taskDef.Agent != "" {
		for _, agent := range agents {
			if agent.Name() == taskDef.Agent {
				assignedAgent = agent
				break
			}
		}
		if assignedAgent == nil {
			return nil, fmt.Errorf("assigned agent %s not found", taskDef.Agent)
		}
	}

	// Convert inputs from config format to core format
	inputs := make(map[string]dive.Input)
	for name, input := range taskDef.Inputs {
		inputs[name] = dive.Input{
			Name:        name,
			Type:        input.Type,
			Description: input.Description,
			Required:    input.Required,
			Default:     input.Default,
		}
	}

	// Convert outputs from config format to core format
	outputs := make(map[string]dive.Output)
	for name, output := range taskDef.Outputs {
		outputs[name] = dive.Output{
			Name:        name,
			Type:        output.Type,
			Description: output.Description,
			Format:      output.Format,
			Default:     output.Default,
		}
	}

	return workflow.NewTask(workflow.TaskOptions{
		Name:        taskDef.Name,
		Description: taskDef.Description,
		Kind:        taskDef.Kind,
		Inputs:      inputs,
		Outputs:     outputs,
		Agent:       assignedAgent,
		Timeout:     timeout,
	}), nil
}

func buildWorkflow(workflowDef Workflow, tasks []*workflow.Task) (*workflow.Workflow, error) {
	tasksByName := make(map[string]*workflow.Task)
	for _, task := range tasks {
		tasksByName[task.Name()] = task
	}
	var foundStart bool

	if len(workflowDef.Steps) == 0 {
		return nil, fmt.Errorf("no steps found")
	}

	steps := []*workflow.Step{}
	for _, step := range workflowDef.Steps {
		task, ok := tasksByName[step.Task]
		if !ok {
			return nil, fmt.Errorf("task %q not found", step.Task)
		}
		var edges []*workflow.Edge
		for _, next := range step.Next {
			var condition workflow.Condition
			if next.Condition != "" {
				var err error
				condition, err = workflow.NewEvalCondition(next.Condition, map[string]any{
					"node": nil,
					"task": nil,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to create condition: %w", err)
				}
			}
			edges = append(edges, &workflow.Edge{
				To:        next.Node,
				Condition: condition,
			})
		}

		// Handle "each" block if one is present
		var each *workflow.EachBlock
		if step.Each != nil {
			each = &workflow.EachBlock{
				Array:         step.Each.Array,
				As:            step.Each.As,
				Parallel:      step.Each.Parallel,
				MaxConcurrent: step.Each.MaxConcurrent,
			}
		}

		// Convert inputs from map[string]string to map[string]interface{}
		inputs := make(map[string]interface{})
		for k, v := range step.With {
			inputs[k] = v
		}

		if step.IsStart {
			if foundStart {
				return nil, fmt.Errorf("multiple start steps found")
			} else {
				foundStart = true
			}
		}

		step := workflow.NewStep(workflow.StepOptions{
			Name:    step.Name,
			Task:    task,
			Next:    edges,
			With:    inputs,
			Each:    each,
			IsStart: step.IsStart,
		})
		steps = append(steps, step)
	}

	// Convert triggers from config to workflow format
	var triggers []*workflow.Trigger
	for _, trigger := range workflowDef.Triggers {
		triggers = append(triggers, &workflow.Trigger{
			Type:   trigger.Type,
			Config: trigger.Config,
		})
	}

	// Set start step if one is not found
	if !foundStart {
		steps[0].SetIsStart(true)
	}

	inputs := make(map[string]dive.Input)
	for name, input := range workflowDef.Inputs {
		inputs[name] = dive.Input{
			Name:        name,
			Type:        input.Type,
			Description: input.Description,
			Required:    input.Required,
			Default:     input.Default,
		}
	}

	outputs := make(map[string]dive.Output)
	for name, output := range workflowDef.Outputs {
		outputs[name] = dive.Output{
			Name:        name,
			Type:        output.Type,
			Description: output.Description,
			Format:      output.Format,
			Default:     output.Default,
		}
	}

	return workflow.NewWorkflow(workflow.WorkflowOptions{
		Name:        workflowDef.Name,
		Description: workflowDef.Description,
		Inputs:      inputs,
		Outputs:     outputs,
		Steps:       steps,
		Triggers:    triggers,
	})
}

func buildTrigger(triggerDef Trigger) (*environment.Trigger, error) {
	return environment.NewTrigger(triggerDef.Name), nil
}
