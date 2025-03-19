package config

import (
	"testing"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/workflow"
	"github.com/stretchr/testify/assert"
)

func TestEnvironment_Build(t *testing.T) {
	// Create a test environment configuration
	env := &Environment{
		Name:        "test-env",
		Description: "Test Environment",
		Config: Config{
			DefaultProvider: "anthropic",
			DefaultModel:    "claude-3-sonnet-20240229",
			LogLevel:        "info",
		},
		Tools: map[string]Tool{
			"google_search": {
				Name:    "google_search",
				Enabled: true,
			},
			"file_read": {
				Name:    "file_read",
				Enabled: true,
			},
		},
		Agents: map[string]AgentConfig{
			"researcher": {
				Name:         "researcher",
				Description:  "Research agent",
				Provider:     "anthropic",
				Model:        "claude-3-sonnet-20240229",
				Tools:        []string{"google_search", "file_read"},
				Instructions: "You are a research agent.",
			},
			"writer": {
				Name:         "writer",
				Description:  "Writing agent",
				Provider:     "anthropic",
				Model:        "claude-3-sonnet-20240229",
				Tools:        []string{"file_read"},
				Instructions: "You are a writing agent.",
			},
		},
		Tasks: map[string]Task{
			"research": {
				Name:        "research",
				Description: "Research task",
				Kind:        "research",
				Agent:       "researcher",
				Inputs: map[string]Input{
					"topic": {
						Type:        "string",
						Description: "Research topic",
						Required:    true,
					},
				},
				Outputs: map[string]Output{
					"findings": {
						Type:        "string",
						Description: "Research findings",
					},
				},
			},
			"write": {
				Name:        "write",
				Description: "Writing task",
				Kind:        "write",
				Agent:       "writer",
				Inputs: map[string]Input{
					"content": {
						Type:        "string",
						Description: "Content to write",
						Required:    true,
					},
				},
				Outputs: map[string]Output{
					"result": {
						Type:        "string",
						Description: "Written content",
					},
				},
			},
		},
		Workflows: map[string]Workflow{
			"research-and-write": {
				Name:        "research-and-write",
				Description: "Research and write workflow",
				Steps: []Step{
					{
						Name: "research-step",
						Task: "research",
						With: map[string]any{
							"topic": "AI technology",
						},
						Next: []NextStep{
							{
								Node: "write-step",
							},
						},
					},
					{
						Name: "write-step",
						Task: "write",
						With: map[string]any{
							"content": "{{node.research-step.outputs.findings}}",
						},
					},
				},
			},
		},
		Triggers: map[string]Trigger{
			"manual": {
				Name: "manual",
				Type: "manual",
			},
		},
	}

	// Build the environment
	result, err := env.Build()
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Verify environment properties
	assert.Equal(t, "test-env", result.Name())
	assert.Equal(t, "Test Environment", result.Description())

	// Verify agents
	agents := result.Agents()
	assert.Len(t, agents, 2)

	// Verify researcher agent
	researcher := findAgentByName(agents, "researcher")
	assert.NotNil(t, researcher)
	assert.False(t, researcher.(*agent.Agent).IsSupervisor())

	// Verify writer agent
	writer := findAgentByName(agents, "writer")
	assert.NotNil(t, writer)
	assert.False(t, writer.(*agent.Agent).IsSupervisor())

	// Verify workflows
	workflows := result.Workflows()
	assert.Len(t, workflows, 1)

	// Verify research-and-write workflow
	researchWorkflow := findWorkflowByName(workflows, "research-and-write")
	assert.NotNil(t, researchWorkflow)
	assert.Equal(t, "Research and write workflow", researchWorkflow.Description())

	// Verify workflow tasks
	tasks := researchWorkflow.Tasks()
	assert.Len(t, tasks, 2)

	// Verify task names
	taskNames := make([]string, len(tasks))
	for i, task := range tasks {
		taskNames[i] = task.Name()
	}
	assert.Contains(t, taskNames, "research")
	assert.Contains(t, taskNames, "write")

	// Verify task assignments
	for _, task := range tasks {
		switch task.Name() {
		case "research":
			assert.Equal(t, "researcher", task.Agent().Name())
		case "write":
			assert.Equal(t, "writer", task.Agent().Name())
		}
	}
}

func findAgentByName(agents []dive.Agent, name string) dive.Agent {
	for _, agent := range agents {
		if agent.Name() == name {
			return agent
		}
	}
	return nil
}

func findWorkflowByName(workflows []*workflow.Workflow, name string) *workflow.Workflow {
	for _, wf := range workflows {
		if wf.Name() == name {
			return wf
		}
	}
	return nil
}
