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
	environment *Environment           // Environment that the execution belongs to
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
	stream      events.Stream          // Stream of events for the execution
}

type ExecutionOptions struct {
	ID          string
	Environment *Environment
	Workflow    *workflow.Workflow
	Status      Status
	StartTime   time.Time
	EndTime     time.Time
	Inputs      map[string]interface{}
	Outputs     map[string]interface{}
	ParentID    string
	ChildIDs    []string
	Metadata    map[string]string
	Logger      slogger.Logger
}

func NewExecution(opts ExecutionOptions) *Execution {
	return &Execution{
		id:          opts.ID,
		environment: opts.Environment,
		workflow:    opts.Workflow,
		status:      opts.Status,
		startTime:   opts.StartTime,
		endTime:     opts.EndTime,
		inputs:      opts.Inputs,
		outputs:     opts.Outputs,
		parentID:    opts.ParentID,
		childIDs:    opts.ChildIDs,
		metadata:    opts.Metadata,
		logger:      opts.Logger,
		taskOutputs: make(map[string]string),
		taskStates:  make(map[string]*taskState),
	}
}

func (e *Execution) ID() string {
	return e.id
}

func (e *Execution) Workflow() *workflow.Workflow {
	return e.workflow
}

func (e *Execution) Environment() *Environment {
	return e.environment
}

func (e *Execution) Status() Status {
	return e.status
}

func (e *Execution) StartTime() time.Time {
	return e.startTime
}

func (e *Execution) Run(ctx context.Context) error {
	if e.status != StatusPending {
		return fmt.Errorf("execution already started")
	}
	e.status = StatusRunning
	e.stream = events.NewStream()

	go func() {
		if err := e.runWorkflow(ctx, e.stream); err != nil {
			e.logger.Error("workflow execution failed", "error", err)
			e.status = StatusFailed
			e.err = err
		}
		e.status = StatusCompleted
		e.endTime = time.Now()
		e.stream.Close()
		e.logger.Info("workflow execution completed", "execution_id", e.id)
	}()

	return nil
}

func (e *Execution) runWorkflow(ctx context.Context, stream events.Stream) error {
	publisher := stream.Publisher()
	defer publisher.Close()

	backgroundCtx := context.Background()

	graph := e.workflow.Graph()
	totalUsage := llm.Usage{}

	e.logger.Debug(
		"workflow execution started",
		"workflow_name", e.workflow.Name(),
		"start_node", graph.Start().Name(),
	)

	node := graph.Start()

	for {
		task := node.Task

		// Determine which agent should take the task
		var agent dive.Agent
		if task.Agent() != nil {
			agent = task.Agent()
		} else {
			// Default to first agent if none assigned
			agent = e.environment.Agents()[0]
		}

		result, err := e.executeTask(ctx, task, agent)
		if err != nil {
			return err
		}
		e.taskOutputs[task.Name()] = result.Content

		nextEdges := node.Next

		if len(nextEdges) == 0 {
			break
		} else if len(nextEdges) == 1 {
			node = nextEdges[0].To
		} else {
			return fmt.Errorf("multiple next edges")
		}
	}

	e.logger.Info(
		"workflow execution completed",
		"workflow_name", e.workflow.Name(),
		"total_usage", totalUsage,
	)
	publisher.Send(backgroundCtx, &events.Event{Type: "workflow.done"})

	return nil
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

func TaskNames(tasks []dive.Task) []string {
	var taskNames []string
	for _, task := range tasks {
		taskNames = append(taskNames, task.Name())
	}
	return taskNames
}
