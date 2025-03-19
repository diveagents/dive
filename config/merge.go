package config

// Merge merges two Environment configs, with the second one taking precedence
func Merge(base, override *Environment) *Environment {

	// Copy base environment
	result := *base

	// Merge name and description if provided
	if override.Name != "" {
		result.Name = override.Name
	}
	if override.Description != "" {
		result.Description = override.Description
	}

	// Merge config
	if override.Config.CacheControl != "" {
		result.Config.CacheControl = override.Config.CacheControl
	}
	if override.Config.DefaultProvider != "" {
		result.Config.DefaultProvider = override.Config.DefaultProvider
	}
	if override.Config.DefaultModel != "" {
		result.Config.DefaultModel = override.Config.DefaultModel
	}
	if override.Config.LogLevel != "" {
		result.Config.LogLevel = override.Config.LogLevel
	}

	// Merge variables
	varMap := make(map[string]Variable)
	for _, v := range result.Variables {
		varMap[v.Name] = v
	}
	for _, v := range override.Variables {
		varMap[v.Name] = v
	}
	result.Variables = varMap

	// Merge tools
	toolMap := make(map[string]Tool)
	for _, t := range result.Tools {
		toolMap[t.Name] = t
	}
	for _, t := range override.Tools {
		toolMap[t.Name] = t
	}
	result.Tools = toolMap

	// Merge agents
	agentMap := make(map[string]AgentConfig)
	for _, agent := range result.Agents {
		agentMap[agent.Name] = agent
	}
	for _, agent := range override.Agents {
		agentMap[agent.Name] = agent
	}
	result.Agents = agentMap

	// Merge tasks
	taskMap := make(map[string]Task)
	for _, task := range result.Tasks {
		taskMap[task.Name] = task
	}
	for _, task := range override.Tasks {
		taskMap[task.Name] = task
	}
	result.Tasks = taskMap

	// Merge workflows
	workflowMap := make(map[string]Workflow)
	for _, workflow := range result.Workflows {
		workflowMap[workflow.Name] = workflow
	}
	for _, workflow := range override.Workflows {
		workflowMap[workflow.Name] = workflow
	}
	result.Workflows = workflowMap

	// Merge documents
	docMap := make(map[string]Document)
	for _, doc := range result.Documents {
		docMap[doc.ID] = doc
	}
	for _, doc := range override.Documents {
		docMap[doc.ID] = doc
	}
	result.Documents = docMap

	// Merge schedules
	scheduleMap := make(map[string]Schedule)
	for _, schedule := range result.Schedules {
		scheduleMap[schedule.Name] = schedule
	}
	for _, schedule := range override.Schedules {
		scheduleMap[schedule.Name] = schedule
	}
	result.Schedules = scheduleMap

	// Merge triggers
	triggerMap := make(map[string]Trigger)
	for _, trigger := range result.Triggers {
		triggerMap[trigger.Name] = trigger
	}
	for _, trigger := range override.Triggers {
		triggerMap[trigger.Name] = trigger
	}
	result.Triggers = triggerMap

	return &result
}
