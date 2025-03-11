package environment

import (
	"context"
	"fmt"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/events"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
)

// Status represents the current state of an execution
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusPaused    Status = "paused"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
)

type taskState struct {
	Task    dive.Task
	Status  dive.TaskStatus
	Started time.Time
	Error   error
}

// Execution represents a single run of a workflow
type Execution struct {
	id          string                 // Unique identifier for this execution
	workflow    *workflow.Workflow     // Workflow being executed
	status      Status                 // Current status of the execution
	startTime   time.Time              // When the execution started
	endTime     time.Time              // When the execution completed (or failed/canceled)
	taskStates  map[string]*taskState  // Current state of each task in the workflow
	taskOutputs map[string]string      // Outputs from each task
	inputs      map[string]interface{} // Input parameters for the workflow
	outputs     map[string]interface{} // Output values from the workflow
	err         error                  // Error if execution failed
	parentID    string                 // ID of parent execution (for sub-executions)
	childIDs    []string               // IDs of child executions
	metadata    map[string]string      // Additional metadata about the execution
	logger      slogger.Logger         // Logger for the execution
}

type ExecutionOptions struct {
	ID        string
	Workflow  *workflow.Workflow
	Status    Status
	StartTime time.Time
	EndTime   time.Time
	Inputs    map[string]interface{}
	Outputs   map[string]interface{}
	Error     error
	ParentID  string
	ChildIDs  []string
	Metadata  map[string]string
	Logger    slogger.Logger
}

func NewExecution(opts ExecutionOptions) *Execution {
	return &Execution{
		id:          opts.ID,
		workflow:    opts.Workflow,
		status:      opts.Status,
		startTime:   opts.StartTime,
		endTime:     opts.EndTime,
		inputs:      opts.Inputs,
		outputs:     opts.Outputs,
		err:         opts.Error,
		parentID:    opts.ParentID,
		childIDs:    opts.ChildIDs,
		metadata:    opts.Metadata,
		logger:      opts.Logger,
		taskOutputs: make(map[string]string),
		taskStates:  make(map[string]*taskState),
	}
}

// Run the execution
func (e *Execution) Run(ctx context.Context, inputs map[string]interface{}) (events.Stream, error) {
	// Validate inputs against workflow.inputs requirements
	// if err := e.workflow.validateInputs(inputs); err != nil {
	// 	return nil, fmt.Errorf("invalid inputs: %w", err)
	// }

	// Convert ordered names back to tasks
	tasksByName := make(map[string]dive.Task, len(e.workflow.Tasks()))
	for _, task := range e.workflow.Tasks() {
		tasksByName[task.Name()] = task
	}
	var orderedTasks []dive.Task
	for _, name := range orderedNames {
		orderedTasks = append(orderedTasks, tasksByName[name])
	}

	// Create stream for workflow events
	stream := events.NewStream()

	// Run workflow execution in background
	go e.executeWorkflow(ctx, orderedTasks, stream)

	return stream, nil
}

func (e *Execution) executeWorkflow(ctx context.Context, tasks []dive.Task, s events.Stream) {
	publisher := s.Publisher()
	defer publisher.Close()

	backgroundCtx := context.Background()
	totalUsage := llm.Usage{}

	e.logger.Debug("workflow execution started",
		"workflow_name", e.workflow.name,
		"task_count", len(tasks),
		"task_names", TaskNames(tasks),
	)

	w := e.workflow

	// Execute tasks sequentially
	for _, task := range tasks {

		// Determine which agent should take the task
		var agent dive.Agent
		if task.AssignedAgent() != nil {
			agent = task.AssignedAgent()
		} else {
			agent = w.agents[0] // Default to first agent if none assigned
		}

		// Execute the task
		result, err := e.executeTask(ctx, task, agent)
		if err != nil {
			publisher.Send(backgroundCtx, &events.Event{
				Type: "workflow.error",
				Origin: events.Origin{
					TaskName:  task.Name(),
					AgentName: agent.Name(),
				},
				Error: err,
			})
			return
		}

		// Store task results
		e.taskOutputs[task.Name()] = result.Content

		publisher.Send(backgroundCtx, &events.Event{
			Type: "workflow.task_done",
			Origin: events.Origin{
				TaskName:  task.Name(),
				AgentName: agent.Name(),
			},
			Payload: result,
		})

		// totalUsage.Add(result.Usage)
	}

	w.logger.Info("workflow execution completed",
		"workflow_name", w.name,
		"total_usage", totalUsage,
	)

	publisher.Send(backgroundCtx, &events.Event{Type: "workflow.done"})
}

func (e *Execution) executeTask(ctx context.Context, task dive.Task, agent dive.Agent) (*dive.TaskResult, error) {
	stream, err := agent.Work(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to start task %q: %w", task.Name(), err)
	}
	defer stream.Close()

	taskResult, err := events.WaitForEvent[*dive.TaskResult](ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for task result: %w", err)
	}
	return taskResult, nil
}

// // Store defines the interface for persisting and retrieving executions
// type Store interface {
// 	// Create a new execution
// 	CreateExecution(ctx context.Context, execution *Execution) error

// 	// Get an execution by ID
// 	GetExecution(ctx context.Context, id string) (*Execution, error)

// 	// Update an existing execution
// 	UpdateExecution(ctx context.Context, execution *Execution) error

// 	// List executions with optional filters
// 	ListExecutions(ctx context.Context, filters map[string]interface{}) ([]*Execution, error)

// 	// Delete an execution and its history
// 	DeleteExecution(ctx context.Context, id string) error
// }

// // Runner manages workflow executions
// type Runner interface {
// 	// Start a new workflow execution
// 	StartExecution(ctx context.Context, workflow *Workflow, inputs map[string]interface{}) (*Execution, error)

// 	// Get the current state of an execution
// 	GetExecutionState(ctx context.Context, executionID string) (*Execution, error)

// 	// Pause a running execution
// 	PauseExecution(ctx context.Context, executionID string) error

// 	// Resume a paused execution
// 	ResumeExecution(ctx context.Context, executionID string) error

// 	// Cancel a running execution
// 	CancelExecution(ctx context.Context, executionID string) error

// 	// List all executions
// 	ListExecutions(ctx context.Context) ([]*Execution, error)
// }

// // Options for configuring a new execution runner
// type RunnerOptions struct {
// 	Store   Store
// 	Logger  slogger.Logger
// 	Metrics MetricsCollector
// }

// // MetricsCollector defines the interface for collecting execution metrics
// type MetricsCollector interface {
// 	// Record the start of an execution
// 	RecordExecutionStart(execution *Execution)

// 	// Record the completion of an execution
// 	RecordExecutionComplete(execution *Execution)

// 	// Record task state changes
// 	RecordTaskStateChange(execution *Execution, taskName string, oldState, newState dive.TaskStatus)

// 	// Record execution errors
// 	RecordExecutionError(execution *Execution, err error)
// }
