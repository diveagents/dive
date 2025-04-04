package config

import (
	"fmt"
	"time"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/agent"
	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/slogger"
)

func buildAgent(
	agentDef Agent,
	config Config,
	tools map[string]llm.Tool,
	logger slogger.Logger,
) (dive.Agent, error) {
	providerName := agentDef.Provider
	if providerName == "" {
		providerName = config.LLM.DefaultProvider
		if providerName == "" {
			providerName = "anthropic"
		}
	}

	modelName := agentDef.Model
	if modelName == "" {
		modelName = config.LLM.DefaultModel
	}

	model, err := GetModel(providerName, modelName)
	if err != nil {
		return nil, fmt.Errorf("error getting model: %w", err)
	}

	var agentTools []llm.Tool
	for _, toolName := range agentDef.Tools {
		tool, ok := tools[toolName]
		if !ok {
			return nil, fmt.Errorf("tool %q not found or not enabled", toolName)
		}
		agentTools = append(agentTools, tool)
	}

	var responseTimeout time.Duration
	if agentDef.ResponseTimeout != "" {
		var err error
		responseTimeout, err = time.ParseDuration(agentDef.ResponseTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid response timeout: %w", err)
		}
	}

	cacheControl := agentDef.CacheControl
	if cacheControl == "" {
		cacheControl = config.LLM.CacheControl
	}

	return agent.New(agent.Options{
		Name:               agentDef.Name,
		Goal:               agentDef.Goal,
		Backstory:          agentDef.Backstory,
		IsSupervisor:       agentDef.IsSupervisor,
		Subordinates:       agentDef.Subordinates,
		Model:              model,
		Tools:              agentTools,
		ResponseTimeout:    responseTimeout,
		ToolIterationLimit: agentDef.ToolIterationLimit,
	})
}
