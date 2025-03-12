package environment

import (
	"context"
	"testing"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/agent"
	"github.com/getstingrai/dive/events"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
	"github.com/stretchr/testify/require"
)

func TestNewEnvironment(t *testing.T) {
	logger := slogger.New(slogger.LevelDebug)

	var tasks []dive.Task

	a := agent.NewMockAgent(agent.MockAgentOptions{
		Name: "Poet Laureate",
		Work: func(ctx context.Context, task dive.Task) (events.Stream, error) {
			tasks = append(tasks, task)
			stream := events.NewStream()
			go func() {
				publisher := stream.Publisher()
				defer publisher.Close()
				var content string
				if task.Name() == "Write a Poem" {
					content = "A haiku about the fall"
				} else if task.Name() == "Write a summary" {
					content = "A summary of that great poem"
				}
				publisher.Send(ctx, &events.Event{
					Type:    "task.result",
					Payload: &dive.TaskResult{Content: content},
				})
			}()
			return stream, nil
		},
	})

	w, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "Poetry Writing",
		Graph: workflow.NewGraph(workflow.GraphOptions{
			Nodes: map[string]*workflow.Node{
				"Write Poem": workflow.NewNode(workflow.NodeOptions{
					IsStart: true,
					Task: workflow.NewTask(workflow.TaskOptions{
						Name:           "Write a Poem",
						ExpectedOutput: "A haiku about the fall",
						Agent:          a,
					}),
					Next: []*workflow.Edge{{To: "Write Summary"}},
				}),
				"Write Summary": workflow.NewNode(workflow.NodeOptions{
					Task: workflow.NewTask(workflow.TaskOptions{
						Name:           "Write a summary",
						ExpectedOutput: "A summary of the poem",
						Agent:          a,
					}),
				}),
			},
		}),
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

	require.Equal(t, 2, len(tasks))
	t1 := tasks[0]
	t2 := tasks[1]
	require.Equal(t, "Write a Poem", t1.Name())
	require.Equal(t, "Write a summary", t2.Name())

	pathStates := execution.PathStates()
	require.Equal(t, 1, len(pathStates))
	s0 := pathStates[0]
	require.Equal(t, PathStatusCompleted, s0.Status)
	require.Equal(t, "Write Summary", s0.CurrentNode.Name())
	require.Equal(t, "A summary of that great poem", s0.NodeOutputs["Write Summary"])
	require.Equal(t, "A haiku about the fall", s0.NodeOutputs["Write Poem"])
}
