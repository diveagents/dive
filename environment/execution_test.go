package environment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
	"github.com/stretchr/testify/require"
)

// mockAgent implements dive.Agent for testing
type mockAgent struct {
	workFn func(ctx context.Context, task dive.Task, inputs map[string]any) (dive.Stream, error)
}

func (m *mockAgent) Name() string {
	return "mock-agent"
}

func (m *mockAgent) Description() string {
	return "Mock agent for testing"
}

func (m *mockAgent) Instructions() string {
	return "Mock instructions"
}

func (m *mockAgent) IsSupervisor() bool {
	return false
}

func (m *mockAgent) SetEnvironment(env dive.Environment) {}

func (m *mockAgent) Generate(ctx context.Context, message *llm.Message, opts ...dive.GenerateOption) (*llm.Response, error) {
	return &llm.Response{}, nil
}

func (m *mockAgent) Stream(ctx context.Context, message *llm.Message, opts ...dive.GenerateOption) (dive.Stream, error) {
	return dive.NewStream(), nil
}

func (m *mockAgent) Work(ctx context.Context, task dive.Task, inputs map[string]any) (dive.Stream, error) {
	if m.workFn != nil {
		return m.workFn(ctx, task, inputs)
	}
	stream := dive.NewStream()
	pub := stream.Publisher()
	pub.Send(ctx, &dive.Event{
		Type: "task.completed",
		Payload: &dive.TaskResult{
			Content: "test output",
		},
	})
	return stream, nil
}

func TestNewExecution(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "test-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name: "test-step",
				Task: workflow.NewTask(workflow.TaskOptions{
					Name:        "test-task",
					Description: "test description",
					Agent:       &mockAgent{},
				}),
			}),
		},
	})
	require.NoError(t, err)

	env := &Environment{}
	exec := NewExecution(ExecutionOptions{
		ID:          "test-exec",
		Environment: env,
		Workflow:    wf,
		Logger:      slogger.NewDevNullLogger(),
	})
	require.Equal(t, "test-exec", exec.ID())
	require.Equal(t, wf, exec.Workflow())
	require.Equal(t, env, exec.Environment())
	require.Equal(t, StatusPending, exec.Status())
}

func TestExecutionBasicFlow(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "test-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:    "test-step",
				IsStart: true,
				Task: workflow.NewTask(workflow.TaskOptions{
					Name:        "test-task",
					Description: "test description",
				}),
			}),
		},
	})
	require.NoError(t, err)

	env, err := New(EnvironmentOptions{
		Name:      "test-env",
		Agents:    []dive.Agent{&mockAgent{}},
		Workflows: []*workflow.Workflow{wf},
		Logger:    slogger.NewDevNullLogger(),
	})
	require.NoError(t, err)

	execution, err := env.StartWorkflow(context.Background(), wf, map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, execution)

	require.NoError(t, execution.Wait())
	require.Equal(t, StatusCompleted, execution.Status())

	outputs := execution.StepOutputs()
	require.Equal(t, "test output", outputs["test-step"])
}

func TestExecutionWithBranching(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "branching-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:    "start",
				IsStart: true,
				Task: workflow.NewTask(workflow.TaskOptions{
					Name: "start-task",
				}),
				Next: []*workflow.Edge{
					{To: "branch1"},
					{To: "branch2"},
				},
			}),
			workflow.NewStep(workflow.StepOptions{
				Name: "branch1",
				Task: workflow.NewTask(workflow.TaskOptions{
					Name: "branch1-task",
				}),
			}),
			workflow.NewStep(workflow.StepOptions{
				Name: "branch2",
				Task: workflow.NewTask(workflow.TaskOptions{
					Name: "branch2-task",
				}),
			}),
		},
	})
	require.NoError(t, err)

	env, err := New(EnvironmentOptions{
		Name:      "test-env",
		Agents:    []dive.Agent{&mockAgent{}},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)

	execution, err := env.StartWorkflow(context.Background(), wf, map[string]interface{}{})
	require.NoError(t, err)

	require.NoError(t, execution.Wait())
	require.Equal(t, StatusCompleted, execution.Status())

	outputs := execution.StepOutputs()
	require.Equal(t, "test output", outputs["start"])
	require.Equal(t, "test output", outputs["branch1"])
	require.Equal(t, "test output", outputs["branch2"])

	stats := execution.GetStats()
	require.Equal(t, 3, stats.TotalPaths)
	require.Equal(t, 0, stats.ActivePaths)
	require.Equal(t, 3, stats.CompletedPaths)
	require.Equal(t, 0, stats.FailedPaths)
}

func TestExecutionWithError(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "error-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:    "error-step",
				IsStart: true,
				Task: workflow.NewTask(workflow.TaskOptions{
					Name: "error-task",
				}),
			}),
		},
	})
	require.NoError(t, err)

	mockAgent := &mockAgent{
		workFn: func(ctx context.Context, task dive.Task, inputs map[string]any) (dive.Stream, error) {
			return nil, fmt.Errorf("simulated error")
		},
	}

	env, err := New(EnvironmentOptions{
		Name:      "test-env",
		Agents:    []dive.Agent{mockAgent},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)

	execution, err := env.StartWorkflow(context.Background(), wf, map[string]interface{}{})
	require.NoError(t, err)

	err = execution.Wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated error")
	require.Equal(t, StatusFailed, execution.Status())

	stats := execution.GetStats()
	require.Equal(t, 1, stats.TotalPaths)
	require.Equal(t, 0, stats.ActivePaths)
	require.Equal(t, 0, stats.CompletedPaths)
	require.Equal(t, 1, stats.FailedPaths)
}

func TestExecutionWithEach(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "each-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:    "each-step",
				IsStart: true,
				Task: workflow.NewTask(workflow.TaskOptions{
					Name:        "each-task",
					Description: "test description",
					Inputs: map[string]dive.Input{
						"item": {
							Type:     "string",
							Required: true,
						},
					},
				}),
				Each: &workflow.EachBlock{
					Array: []string{"item1", "item2", "item3"},
					As:    "item",
				},
			}),
		},
	})
	require.NoError(t, err)

	var processedItems []string
	mockAgent := &mockAgent{
		workFn: func(ctx context.Context, task dive.Task, inputs map[string]any) (dive.Stream, error) {
			item, ok := inputs["item"]
			if !ok {
				return nil, fmt.Errorf("missing required input: item")
			}
			itemStr, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("invalid type for input 'item': expected string, got %T", item)
			}
			processedItems = append(processedItems, itemStr)

			stream := dive.NewStream()
			pub := stream.Publisher()
			pub.Send(ctx, &dive.Event{
				Type: "task.completed",
				Payload: &dive.TaskResult{
					Content: fmt.Sprintf("processed %s", itemStr),
				},
			})
			return stream, nil
		},
	}

	env, err := New(EnvironmentOptions{
		Name:      "test-env",
		Agents:    []dive.Agent{mockAgent},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)

	execution, err := env.StartWorkflow(context.Background(), wf, map[string]interface{}{})
	require.NoError(t, err)

	require.NoError(t, execution.Wait())
	require.Equal(t, StatusCompleted, execution.Status())
	require.ElementsMatch(t, []string{"item1", "item2", "item3"}, processedItems)
}

func TestExecutionWithInputs(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "input-workflow",
		Inputs: map[string]dive.Input{
			"required_input": {
				Type:     "string",
				Required: true,
			},
			"optional_input": {
				Type:    "string",
				Default: "default_value",
			},
		},
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:    "input-step",
				IsStart: true,
				Task: workflow.NewTask(workflow.TaskOptions{
					Name: "input-task",
				}),
			}),
		},
	})
	require.NoError(t, err)

	env, err := New(EnvironmentOptions{
		Name:      "test-env",
		Agents:    []dive.Agent{&mockAgent{}},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)

	// Test missing required input
	_, err = env.StartWorkflow(context.Background(), wf, map[string]interface{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required input")

	// Test with required input
	execution, err := env.StartWorkflow(context.Background(), wf, map[string]interface{}{
		"required_input": "test_value",
	})
	require.NoError(t, err)
	require.NoError(t, execution.Wait())
	require.Equal(t, StatusCompleted, execution.Status())
}

func TestExecutionContextCancellation(t *testing.T) {
	wf, err := workflow.NewWorkflow(workflow.WorkflowOptions{
		Name: "cancellation-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:    "slow-step",
				IsStart: true,
				Task: workflow.NewTask(workflow.TaskOptions{
					Name: "slow-task",
				}),
			}),
		},
	})
	require.NoError(t, err)

	mockAgent := &mockAgent{
		workFn: func(ctx context.Context, task dive.Task, inputs map[string]any) (dive.Stream, error) {
			stream := dive.NewStream()
			go func() {
				time.Sleep(100 * time.Millisecond)
				pub := stream.Publisher()
				pub.Send(ctx, &dive.Event{
					Type: "task.completed",
					Payload: &dive.TaskResult{
						Content: "completed",
					},
				})
			}()
			return stream, nil
		},
	}

	env, err := New(EnvironmentOptions{
		Name:      "test-env",
		Agents:    []dive.Agent{mockAgent},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	execution, err := env.StartWorkflow(ctx, wf, map[string]interface{}{})
	require.NoError(t, err)

	// Cancel the context before the task completes
	cancel()

	err = execution.Wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")
	require.Equal(t, StatusFailed, execution.Status())
}
