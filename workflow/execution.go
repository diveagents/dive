package workflow

import (
	"context"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/slogger"
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

// Execution represents a single run of a workflow
type Execution struct {
	ID           string                 // Unique identifier for this execution
	WorkflowName string                 // Name of the workflow being executed
	Status       Status                 // Current status of the execution
	StartTime    time.Time              // When the execution started
	EndTime      time.Time              // When the execution completed (or failed/canceled)
	TaskStates   map[string]TaskState   // Current state of each task in the workflow
	Inputs       map[string]interface{} // Input parameters for the workflow
	Outputs      map[string]interface{} // Output values from the workflow
	Error        error                  // Error if execution failed
	ParentID     string                 // ID of parent execution (for sub-executions)
	ChildIDs     []string               // IDs of child executions
	Metadata     map[string]string      // Additional metadata about the execution
}

// TaskState tracks the state of a single task within an execution
type TaskState struct {
	TaskName     string
	Status       dive.TaskStatus
	StartTime    time.Time
	EndTime      time.Time
	Error        error
	RetryCount   int
	LastAttempt  time.Time
	Dependencies []string
	Outputs      map[string]interface{}
}

// Store defines the interface for persisting and retrieving executions
type Store interface {
	// Create a new execution
	CreateExecution(ctx context.Context, execution *Execution) error

	// Get an execution by ID
	GetExecution(ctx context.Context, id string) (*Execution, error)

	// Update an existing execution
	UpdateExecution(ctx context.Context, execution *Execution) error

	// List executions with optional filters
	ListExecutions(ctx context.Context, filters map[string]interface{}) ([]*Execution, error)

	// Delete an execution and its history
	DeleteExecution(ctx context.Context, id string) error
}

// Runner manages workflow executions
type Runner interface {
	// Start a new workflow execution
	StartExecution(ctx context.Context, workflow *Workflow, inputs map[string]interface{}) (*Execution, error)

	// Get the current state of an execution
	GetExecutionState(ctx context.Context, executionID string) (*Execution, error)

	// Pause a running execution
	PauseExecution(ctx context.Context, executionID string) error

	// Resume a paused execution
	ResumeExecution(ctx context.Context, executionID string) error

	// Cancel a running execution
	CancelExecution(ctx context.Context, executionID string) error

	// List all executions
	ListExecutions(ctx context.Context) ([]*Execution, error)
}

// Options for configuring a new execution runner
type RunnerOptions struct {
	Store   Store
	Logger  slogger.Logger
	Metrics MetricsCollector
}

// MetricsCollector defines the interface for collecting execution metrics
type MetricsCollector interface {
	// Record the start of an execution
	RecordExecutionStart(execution *Execution)

	// Record the completion of an execution
	RecordExecutionComplete(execution *Execution)

	// Record task state changes
	RecordTaskStateChange(execution *Execution, taskName string, oldState, newState dive.TaskStatus)

	// Record execution errors
	RecordExecutionError(execution *Execution, err error)
}
