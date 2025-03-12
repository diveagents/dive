package environment

import (
	"testing"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/workflow"
	"github.com/stretchr/testify/require"
)

func TestNewEnvironment(t *testing.T) {

	a := agent.NewAgent(agent.AgentOptions{
		Name: "Poet Laureate",
	})

	writePoem := workflow.NewTask(workflow.TaskOptions{
		Name:           "Write a Poem",
		ExpectedOutput: "A haiku about the fall",
		Agent:          a,
	})

	graph, err := workflow.NewGraph(map[string]*workflow.Node{
		"Write": {
			Task: writePoem,
		},
	}, "Write")

	require.NoError(t, err)

	w, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name:  "Poetry Writing",
		Tasks: []dive.Task{writePoem},
		Graph: graph,
	})
	require.NoError(t, err)

	env, err := New(EnvironmentOptions{
		Name:      "test",
		Agents:    []dive.Agent{},
		Workflows: []*workflow.Workflow{w},
	})
	require.NoError(t, err)
	require.NotNil(t, env)

	require.Equal(t, "test", env.Name())
}
