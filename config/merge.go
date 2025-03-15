package config

import "sort"

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

	// Merge variables (by name)
	varMap := make(map[string]Variable)
	for _, v := range result.Variables {
		varMap[v.Name] = v
	}
	for _, v := range override.Variables {
		varMap[v.Name] = v
	}
	result.Variables = make([]Variable, 0, len(varMap))
	for _, v := range varMap {
		result.Variables = append(result.Variables, v)
	}
	sort.Slice(result.Variables, func(i, j int) bool {
		return result.Variables[i].Name < result.Variables[j].Name
	})

	// Merge tools (by name)
	toolMap := make(map[string]Tool)
	for _, t := range result.Tools {
		toolMap[t.Name] = t
	}
	for _, t := range override.Tools {
		toolMap[t.Name] = t
	}
	result.Tools = make([]Tool, 0, len(toolMap))
	for _, t := range toolMap {
		result.Tools = append(result.Tools, t)
	}
	sort.Slice(result.Tools, func(i, j int) bool {
		return result.Tools[i].Name < result.Tools[j].Name
	})

	// Merge agents (by name)
	agentMap := make(map[string]AgentConfig)
	for _, agent := range result.Agents {
		agentMap[agent.Name] = agent
	}
	for _, agent := range override.Agents {
		agentMap[agent.Name] = agent
	}
	result.Agents = make([]AgentConfig, 0, len(agentMap))
	for _, agent := range agentMap {
		result.Agents = append(result.Agents, agent)
	}
	sort.Slice(result.Agents, func(i, j int) bool {
		return result.Agents[i].Name < result.Agents[j].Name
	})

	// Merge tasks (by name)
	taskMap := make(map[string]Task)
	for _, task := range result.Tasks {
		taskMap[task.Name] = task
	}
	for _, task := range override.Tasks {
		taskMap[task.Name] = task
	}
	result.Tasks = make([]Task, 0, len(taskMap))
	for _, task := range taskMap {
		result.Tasks = append(result.Tasks, task)
	}
	sort.Slice(result.Tasks, func(i, j int) bool {
		return result.Tasks[i].Name < result.Tasks[j].Name
	})

	// Merge workflows (by name)
	workflowMap := make(map[string]Workflow)
	for _, workflow := range result.Workflows {
		workflowMap[workflow.Name] = workflow
	}
	for _, workflow := range override.Workflows {
		workflowMap[workflow.Name] = workflow
	}
	result.Workflows = make([]Workflow, 0, len(workflowMap))
	for _, workflow := range workflowMap {
		result.Workflows = append(result.Workflows, workflow)
	}
	sort.Slice(result.Workflows, func(i, j int) bool {
		return result.Workflows[i].Name < result.Workflows[j].Name
	})

	// Merge documents (by ID)
	docMap := make(map[string]Document)
	for _, doc := range result.Documents {
		docMap[doc.ID] = doc
	}
	for _, doc := range override.Documents {
		docMap[doc.ID] = doc
	}
	result.Documents = make([]Document, 0, len(docMap))
	for _, doc := range docMap {
		result.Documents = append(result.Documents, doc)
	}
	sort.Slice(result.Documents, func(i, j int) bool {
		return result.Documents[i].ID < result.Documents[j].ID
	})

	// Merge schedules (by name)
	scheduleMap := make(map[string]Schedule)
	for _, schedule := range result.Schedules {
		scheduleMap[schedule.Name] = schedule
	}
	for _, schedule := range override.Schedules {
		scheduleMap[schedule.Name] = schedule
	}
	result.Schedules = make([]Schedule, 0, len(scheduleMap))
	for _, schedule := range scheduleMap {
		result.Schedules = append(result.Schedules, schedule)
	}
	sort.Slice(result.Schedules, func(i, j int) bool {
		return result.Schedules[i].Name < result.Schedules[j].Name
	})

	// Merge triggers (by name)
	triggerMap := make(map[string]Trigger)
	for _, trigger := range result.Triggers {
		triggerMap[trigger.Name] = trigger
	}
	for _, trigger := range override.Triggers {
		triggerMap[trigger.Name] = trigger
	}
	result.Triggers = make([]Trigger, 0, len(triggerMap))
	for _, trigger := range triggerMap {
		result.Triggers = append(result.Triggers, trigger)
	}
	sort.Slice(result.Triggers, func(i, j int) bool {
		return result.Triggers[i].Name < result.Triggers[j].Name
	})

	return &result
}
