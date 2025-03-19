package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/environment"
	"github.com/getstingrai/dive/workflow"
	"gopkg.in/yaml.v3"
)

// Environment is a serializable representation of an AI agent environment
type Environment struct {
	Name        string                 `yaml:"name,omitempty" json:"name,omitempty"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Config      Config                 `yaml:"config,omitempty" json:"config,omitempty"`
	Variables   map[string]Variable    `yaml:"variables,omitempty" json:"variables,omitempty"`
	Tools       map[string]Tool        `yaml:"tools,omitempty" json:"tools,omitempty"`
	Documents   map[string]Document    `yaml:"documents,omitempty" json:"documents,omitempty"`
	Agents      map[string]AgentConfig `yaml:"agents,omitempty" json:"agents,omitempty"`
	Tasks       map[string]Task        `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	Workflows   map[string]Workflow    `yaml:"workflows,omitempty" json:"workflows,omitempty"`
	Triggers    map[string]Trigger     `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	Schedules   map[string]Schedule    `yaml:"schedules,omitempty" json:"schedules,omitempty"`
}

// Save writes an Environment configuration to a file. The file extension is used to
// determine the configuration format:
// - .json -> JSON
// - .yml or .yaml -> YAML
func (env *Environment) Save(path string) error {
	// Determine format from extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return env.SaveJSON(path)
	case ".yml", ".yaml":
		return env.SaveYAML(path)
	default:
		return fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// SaveYAML writes an Environment configuration to a YAML file
func (env *Environment) SaveYAML(path string) error {
	data, err := yaml.Marshal(env)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SaveJSON writes an Environment configuration to a JSON file
func (env *Environment) SaveJSON(path string) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Write an Environment configuration to a writer in YAML format
func (env *Environment) Write(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(env)
}

// Build creates a new Environment from the configuration
func (env *Environment) Build(opts ...BuildOption) (*environment.Environment, error) {
	buildOpts := &BuildOptions{}
	for _, opt := range opts {
		opt(buildOpts)
	}

	// Tools
	var toolConfigs map[string]map[string]interface{}
	if env.Tools != nil {
		toolConfigs = make(map[string]map[string]interface{}, len(env.Tools))
		for name, tool := range env.Tools {
			toolName := name
			if tool.Name != "" {
				toolName = tool.Name
			}
			toolConfigs[toolName] = map[string]interface{}{
				"enabled": tool.Enabled,
			}
		}
	}
	toolsMap, err := initializeTools(toolConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools: %w", err)
	}

	// Agents
	agents := make([]dive.Agent, 0, len(env.Agents))
	for name, agentDef := range env.Agents {
		if agentDef.Name == "" {
			agentDef.Name = name
		}
		agent, err := buildAgent(agentDef, env.Config, toolsMap, buildOpts.Logger, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", agentDef.Name, err)
		}
		agents = append(agents, agent)
	}

	// Tasks
	var tasks []*workflow.Task
	for name, taskDef := range env.Tasks {
		if taskDef.Name == "" {
			taskDef.Name = name
		}
		task, err := buildTask(taskDef, agents, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build task %s: %w", taskDef.Name, err)
		}
		tasks = append(tasks, task)
	}

	// Workflows
	var workflows []*workflow.Workflow
	for name, workflowDef := range env.Workflows {
		if workflowDef.Name == "" {
			workflowDef.Name = name
		}
		workflow, err := buildWorkflow(workflowDef, tasks)
		if err != nil {
			return nil, fmt.Errorf("failed to build workflow %s: %w", workflowDef.Name, err)
		}
		workflows = append(workflows, workflow)
	}

	// Triggers
	var triggers []*environment.Trigger
	for name, triggerDef := range env.Triggers {
		if triggerDef.Name == "" {
			triggerDef.Name = name
		}
		trigger, err := buildTrigger(triggerDef)
		if err != nil {
			return nil, fmt.Errorf("failed to build trigger %s: %w", triggerDef.Name, err)
		}
		triggers = append(triggers, trigger)
	}

	// Environment
	result, err := environment.New(environment.EnvironmentOptions{
		Name:        env.Name,
		Description: env.Description,
		Agents:      agents,
		Workflows:   workflows,
		Triggers:    triggers,
		Logger:      buildOpts.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}
	return result, nil
}
