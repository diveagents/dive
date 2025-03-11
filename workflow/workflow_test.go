package workflow

import (
	"context"
	"testing"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAgent implements dive.Agent for testing
type MockAgent struct {
	mock.Mock
}

func (m *MockAgent) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgent) Description() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgent) Instructions() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgent) IsSupervisor() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockAgent) Subordinates() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockAgent) Work(ctx context.Context, task dive.Task) (events.Stream, error) {
	args := m.Called(ctx, task)
	return args.Get(0).(events.Stream), args.Error(1)
}

func TestNewWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		opts    WorkflowOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid workflow",
			opts: WorkflowOptions{
				Name:        "test-workflow",
				Description: "Test workflow",
				Agents:      []dive.Agent{&MockAgent{}},
				Repository:  &document.MemoryRepository{},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			opts: WorkflowOptions{
				Description: "Test workflow",
				Agents:      []dive.Agent{&MockAgent{}},
				Repository:  &document.MemoryRepository{},
			},
			wantErr: true,
			errMsg:  "workflow name required",
		},
		{
			name: "missing agents",
			opts: WorkflowOptions{
				Name:        "test-workflow",
				Description: "Test workflow",
				Repository:  &document.MemoryRepository{},
			},
			wantErr: true,
			errMsg:  "at least one agent required",
		},
		{
			name: "missing repository",
			opts: WorkflowOptions{
				Name:        "test-workflow",
				Description: "Test workflow",
				Agents:      []dive.Agent{&MockAgent{}},
			},
			wantErr: true,
			errMsg:  "document repository required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock expectations for all test cases
			if len(tt.opts.Agents) > 0 {
				mockAgent := tt.opts.Agents[0].(*MockAgent)
				mockAgent.On("Name").Return("test-agent")
				mockAgent.On("Description").Return("test agent")
				mockAgent.On("Instructions").Return("test instructions")
				mockAgent.On("IsSupervisor").Return(true)
			}

			workflow, err := NewWorkflow(tt.opts)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, workflow)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, workflow)
				assert.Equal(t, tt.opts.Name, workflow.Name())
				assert.Equal(t, tt.opts.Description, workflow.Description())
			}
		})
	}
}

func TestWorkflow_Validate(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *Workflow
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid workflow",
			setup: func() *Workflow {
				return &Workflow{
					name:        "test",
					description: "test workflow",
					tasks: []dive.Task{
						NewTask(TaskOptions{
							Name:        "task1",
							Description: "Task 1",
						}),
					},
				}
			},
			wantErr: false,
		},
		{
			name: "missing name",
			setup: func() *Workflow {
				return &Workflow{
					description: "test workflow",
					tasks: []dive.Task{
						NewTask(TaskOptions{
							Name:        "task1",
							Description: "Task 1",
						}),
					},
				}
			},
			wantErr: true,
			errMsg:  "workflow name required",
		},
		{
			name: "missing description",
			setup: func() *Workflow {
				return &Workflow{
					name: "test",
					tasks: []dive.Task{
						NewTask(TaskOptions{
							Name:        "task1",
							Description: "Task 1",
						}),
					},
				}
			},
			wantErr: true,
			errMsg:  "workflow description required",
		},
		{
			name: "no tasks",
			setup: func() *Workflow {
				return &Workflow{
					name:        "test",
					description: "test workflow",
				}
			},
			wantErr: true,
			errMsg:  "workflow must have at least one task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := tt.setup()
			err := workflow.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWorkflow_Execute(t *testing.T) {
	// Setup test components
	mockAgent := &MockAgent{}
	mockRepo := &document.MemoryRepository{}

	task := NewTask(TaskOptions{
		Name:        "task1",
		Description: "Task 1",
	})

	stream := events.NewStream()

	mockAgent.On("Name").Return("test-agent")
	mockAgent.On("Description").Return("test agent")
	mockAgent.On("Instructions").Return("test instructions")
	mockAgent.On("IsSupervisor").Return(true)
	mockAgent.On("Work", mock.Anything, task).Return(stream, nil)

	// Create workflow
	workflow, err := NewWorkflow(WorkflowOptions{
		Name:        "test",
		Description: "test workflow",
		Agents:      []dive.Agent{mockAgent},
		Repository:  mockRepo,
		Tasks:       []dive.Task{task},
	})
	require.NoError(t, err)

	// Execute workflow
	resultStream, err := workflow.Execute(context.Background(), map[string]interface{}{})

	// Assert results
	require.NoError(t, err)
	assert.NotNil(t, resultStream)
}

// func TestWorkflow_ExecuteTask(t *testing.T) {
// 	ctx := context.Background()
// 	mockAgent := &MockAgent{}
// 	mockPublisher := events.NewStream().Publisher()

// 	task := NewTask(TaskOptions{
// 		Name:        "test-task",
// 		Description: "Test task",
// 	})

// 	tests := []struct {
// 		name    string
// 		setup   func()
// 		wantErr bool
// 		errMsg  string
// 	}{
// 		{
// 			name: "successful task execution",
// 			setup: func() {
// 				stream := events.NewStream()
// 				go func() {
// 					publisher := stream.Publisher()
// 					publisher.Send(ctx, &dive.TaskResult{
// 						Content: "test result",
// 					})
// 					publisher.Close()
// 				}()
// 				mockAgent.On("Name").Return("test-agent")
// 				mockAgent.On("Description").Return("test agent")
// 				mockAgent.On("Instructions").Return("test instructions")
// 				mockAgent.On("IsSupervisor").Return(true)
// 				mockAgent.On("Work", mock.Anything, task).Return(stream, nil).Once()
// 			},
// 			wantErr: false,
// 		},
// 		{
// 			name: "task execution error",
// 			setup: func() {
// 				mockAgent.On("Work", mock.Anything, task).Return(events.NewStream(), assert.AnError).Once()
// 			},
// 			wantErr: true,
// 			errMsg:  "failed to start task",
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			tt.setup()

// 			workflow := &Workflow{}
// 			result, err := workflow.executeTask(ctx, task, mockAgent, mockPublisher)

// 			if tt.wantErr {
// 				require.Error(t, err)
// 				assert.Contains(t, err.Error(), tt.errMsg)
// 				assert.Nil(t, result)
// 			} else {
// 				require.NoError(t, err)
// 				assert.NotNil(t, result)
// 				assert.Equal(t, "test result", result.Content)
// 			}
// 		})
// 	}
// }
