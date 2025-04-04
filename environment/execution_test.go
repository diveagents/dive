package environment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/slogger"
	"github.com/diveagents/dive/workflow"
	"github.com/stretchr/testify/require"
)

// mockAgent implements dive.Agent for testing
type mockAgent struct {
	workFn func(ctx context.Context, task dive.Task) (dive.EventStream, error)
}

func (m *mockAgent) Name() string {
	return "mock-agent"
}

func (m *mockAgent) Goal() string {
	return "Mock agent for testing"
}

func (m *mockAgent) Backstory() string {
	return "Mock backstory"
}

func (m *mockAgent) IsSupervisor() bool {
	return false
}

func (m *mockAgent) SetEnvironment(env dive.Environment) error {
	return nil
}

func (m *mockAgent) Chat(ctx context.Context, messages []*llm.Message, opts ...dive.ChatOption) (dive.EventStream, error) {
	return m.workFn(ctx, nil)
}

func (m *mockAgent) Work(ctx context.Context, task dive.Task) (dive.EventStream, error) {
	if m.workFn != nil {
		return m.workFn(ctx, task)
	}
	stream, publisher := dive.NewEventStream()
	publisher.Send(ctx, &dive.Event{
		Type: "task.completed",
		Payload: &dive.TaskResult{
			Content: "test output",
		},
	})
	publisher.Close()
	return stream, nil
}

func TestNewExecution(t *testing.T) {
	wf, err := workflow.New(workflow.Options{
		Name: "test-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:  "test-step",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "test-task",
					Text: "test description",
				},
			}),
		},
	})
	require.NoError(t, err)

	env := &Environment{}
	require.NoError(t, env.Start(context.Background()))
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
	wf, err := workflow.New(workflow.Options{
		Name: "test-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:  "test-step",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "test-task",
					Text: "test description",
				},
			}),
		},
	})
	require.NoError(t, err)

	env, err := New(Options{
		Name:      "test-env",
		Agents:    []dive.Agent{&mockAgent{}},
		Workflows: []*workflow.Workflow{wf},
		Logger:    slogger.NewDevNullLogger(),
	})
	require.NoError(t, err)
	require.NoError(t, env.Start(context.Background()))

	execution, err := env.ExecuteWorkflow(context.Background(), wf.Name(), map[string]interface{}{})
	require.NoError(t, err)
	require.NotNil(t, execution)

	require.NoError(t, execution.Wait())
	require.Equal(t, StatusCompleted, execution.Status())

	outputs := execution.StepOutputs()
	require.Equal(t, "test output", outputs["test-step"])
}

func TestExecutionWithBranching(t *testing.T) {
	wf, err := workflow.New(workflow.Options{
		Name: "branching-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:  "start",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "start-task",
					Text: "Start Task",
				},
				Next: []*workflow.Edge{
					{Step: "branch1"},
					{Step: "branch2"},
				},
			}),
			workflow.NewStep(workflow.StepOptions{
				Name:  "branch1",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "branch1-task",
					Text: "Branch 1 Task",
				},
			}),
			workflow.NewStep(workflow.StepOptions{
				Name:  "branch2",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "branch2-task",
					Text: "Branch 2 Task",
				},
			}),
		},
	})
	require.NoError(t, err)

	env, err := New(Options{
		Name:      "test-env",
		Agents:    []dive.Agent{&mockAgent{}},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)
	require.NoError(t, env.Start(context.Background()))

	execution, err := env.ExecuteWorkflow(context.Background(), wf.Name(), map[string]interface{}{})
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
	wf, err := workflow.New(workflow.Options{
		Name: "error-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name: "error-step",
				Prompt: &dive.Prompt{
					Name: "error-task",
					Text: "Error Task",
				},
			}),
		},
	})
	require.NoError(t, err)

	mockAgent := &mockAgent{
		workFn: func(ctx context.Context, task dive.Task) (dive.EventStream, error) {
			return nil, fmt.Errorf("simulated error")
		},
	}

	env, err := New(Options{
		Name:      "test-env",
		Agents:    []dive.Agent{mockAgent},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)
	require.NoError(t, env.Start(context.Background()))

	execution, err := env.ExecuteWorkflow(context.Background(), wf.Name(), map[string]interface{}{})
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

func TestExecutionWithInputs(t *testing.T) {
	wf, err := workflow.New(workflow.Options{
		Name: "input-workflow",
		Inputs: []*dive.Input{
			{
				Name:     "required_input",
				Type:     "string",
				Required: true,
			},
			{
				Name:     "optional_input",
				Type:     "string",
				Default:  "default_value",
				Required: false,
			},
		},
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:  "input-step",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "input-task",
					Text: "Input Task",
				},
			}),
		},
	})
	require.NoError(t, err)

	env, err := New(Options{
		Name:      "test-env",
		Agents:    []dive.Agent{&mockAgent{}},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)
	require.NoError(t, env.Start(context.Background()))

	// Test missing required input
	_, err = env.ExecuteWorkflow(context.Background(), wf.Name(), map[string]interface{}{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "required input")

	// Test with required input
	execution, err := env.ExecuteWorkflow(context.Background(), wf.Name(), map[string]interface{}{
		"required_input": "test_value",
	})
	require.NoError(t, err)
	require.NoError(t, execution.Wait())
	require.Equal(t, StatusCompleted, execution.Status())
}

func TestExecutionContextCancellation(t *testing.T) {
	wf, err := workflow.New(workflow.Options{
		Name: "cancellation-workflow",
		Steps: []*workflow.Step{
			workflow.NewStep(workflow.StepOptions{
				Name:  "slow-step",
				Agent: &mockAgent{},
				Prompt: &dive.Prompt{
					Name: "slow-task",
					Text: "Slow Task",
				},
			}),
		},
	})
	require.NoError(t, err)

	mockAgent := &mockAgent{
		workFn: func(ctx context.Context, task dive.Task) (dive.EventStream, error) {
			stream, publisher := dive.NewEventStream()
			go func() {
				defer publisher.Close()
				time.Sleep(100 * time.Millisecond)
				publisher.Send(ctx, &dive.Event{
					Type: "task.completed",
					Payload: &dive.TaskResult{
						Content: "completed",
					},
				})
			}()
			return stream, nil
		},
	}

	env, err := New(Options{
		Name:      "test-env",
		Agents:    []dive.Agent{mockAgent},
		Workflows: []*workflow.Workflow{wf},
	})
	require.NoError(t, err)
	require.NoError(t, env.Start(context.Background()))

	ctx, cancel := context.WithCancel(context.Background())
	execution, err := env.ExecuteWorkflow(ctx, wf.Name(), map[string]interface{}{})
	require.NoError(t, err)

	// Cancel the context before the task completes
	cancel()

	err = execution.Wait()
	require.Error(t, err)
	require.Contains(t, err.Error(), "context canceled")
	require.Equal(t, StatusFailed, execution.Status())
}
