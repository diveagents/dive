package teamconf

import (
	"fmt"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/document"
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

func WithLogLevel(level string) BuildOption {
	return func(opts *BuildOptions) {
		opts.LogLevel = level
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

func buildAgent(
	agentDef agent.Agent,
	globalConfig agent.Config,
	toolsMap map[string]llm.Tool,
	logger slogger.Logger,
	variables map[string]interface{},
) (dive.Agent, error) {

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

	var taskTimeout, chatTimeout time.Duration
	if agentDef.TaskTimeout != "" {
		var err error
		taskTimeout, err = time.ParseDuration(agentDef.TaskTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid task timeout: %w", err)
		}
	}

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

	agent := agent.NewAgent(agent.AgentOptions{
		Name:               agentDef.Name,
		Description:        agentDef.Description,
		Instructions:       agentDef.Instructions,
		IsSupervisor:       agentDef.IsSupervisor,
		Subordinates:       agentDef.Subordinates,
		AcceptedEvents:     agentDef.AcceptedEvents,
		LLM:                llmProvider,
		Tools:              agentTools,
		TaskTimeout:        taskTimeout,
		ChatTimeout:        chatTimeout,
		CacheControl:       cacheControl,
		LogLevel:           globalConfig.LogLevel,
		Logger:             logger,
		ToolIterationLimit: agentDef.ToolIterationLimit,
	})
	return agent, nil
}

func buildTask(
	taskDef workflow.Task,
	agents []dive.Agent,
	variables map[string]interface{},
) (*workflow.Task, error) {
	var timeout time.Duration
	if stepDef.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(taskDef.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
	}

	// Find assigned agent if specified
	var assignedAgent dive.Agent
	if stepDef.AssignedAgent != "" {
		for _, agent := range agents {
			if agent.Name() == stepDef.AssignedAgent {
				assignedAgent = agent
				break
			}
		}
		if assignedAgent == nil {
			return nil, fmt.Errorf("assigned agent %s not found", stepDef.AssignedAgent)
		}
	}

	// Convert document names to document refs
	var documentRefs []document.DocumentRef
	for _, docName := range stepDef.Documents {
		documentRefs = append(documentRefs, document.DocumentRef{
			Name: docName,
		})
	}

	return workflow.NewStep(workflow.StepOptions{
		Name:           stepDef.Name,
		Description:    stepDef.Description,
		ExpectedOutput: stepDef.ExpectedOutput,
		Dependencies:   stepDef.Dependencies,
		OutputFormat:   workflow.OutputFormat(stepDef.OutputFormat),
		AssignedAgent:  assignedAgent,
		OutputFile:     stepDef.OutputFile,
		Timeout:        timeout,
		Context:        stepDef.Context,
		DocumentRefs:   documentRefs,
	}), nil
}
