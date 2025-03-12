package environment

import (
	"context"
	"testing"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
	"github.com/stretchr/testify/require"
)

func TestNewEnvironment(t *testing.T) {
	logger := slogger.New(slogger.LevelDebug)

	a := agent.NewAgent(agent.AgentOptions{
		Name:   "Poet Laureate",
		Logger: logger,
	})

	writePoem := workflow.NewTask(workflow.TaskOptions{
		Name:           "Write a Poem",
		ExpectedOutput: "A haiku about the fall",
		Agent:          a,
	})

	graph := workflow.NewGraph(workflow.GraphOptions{
		Nodes: map[string]*workflow.Node{
			"Write Poem": workflow.NewNode(workflow.NodeOptions{
				Task:    writePoem,
				IsStart: true,
			}),
		},
	})

	w, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name:  "Poetry Writing",
		Tasks: []dive.Task{writePoem},
		Graph: graph,
	})
	require.NoError(t, err)

	env, err := New(EnvironmentOptions{
		Name:      "test",
		Agents:    []dive.Agent{a},
		Workflows: []*workflow.Workflow{w},
		Logger:    logger,
	})
	require.NoError(t, err)
	require.NotNil(t, env)

	require.Equal(t, "test", env.Name())

	execution, err := env.StartWorkflow(context.Background(), w, map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, execution)

	err = execution.Wait()
	require.NoError(t, err)

	require.True(t, false)
}
