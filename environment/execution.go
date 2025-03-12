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

type executionPath struct {
	id          string
	currentNode *workflow.Node
}

type pathUpdate struct {
	pathID     string
	taskOutput string
	newPaths   []*executionPath
	err        error
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
		return fmt.Errorf("unexpected execution status: %s", e.status)
	}
	e.status = StatusRunning
	err := e.runWorkflow(ctx)
	e.endTime = time.Now()
	if err != nil {
		e.logger.Error("workflow execution failed", "error", err)
		e.status = StatusFailed
		e.err = err
		return err
	}
	e.logger.Info("workflow execution completed", "execution_id", e.id)
	e.status = StatusCompleted
	e.err = nil
	return nil
}

func (e *Execution) runWorkflow(ctx context.Context) error {

	graph := e.workflow.Graph()
	totalUsage := llm.Usage{}

	e.logger.Info(
		"workflow execution started",
		"workflow_name", e.workflow.Name(),
		"start_node", graph.Start()[0].Name(),
	)

	// Channel for path updates
	updates := make(chan pathUpdate)
	activePaths := make(map[string]*executionPath)

	// Start initial paths
	for i, startNode := range graph.Start() {
		initialPath := &executionPath{
			id:          fmt.Sprintf("path-%d", i+1),
			currentNode: startNode,
		}
		activePaths[initialPath.id] = initialPath
		go e.runPath(ctx, initialPath, updates)
		e.logger.Info("started initial path", "path_id", initialPath.id)
	}

	// Main control loop
	for len(activePaths) > 0 {
		e.logger.Info("active paths", "active_paths", len(activePaths))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.err != nil {
				return fmt.Errorf("path %s failed: %w", update.pathID, update.err)
			}

			// Store task output
			path := activePaths[update.pathID]
			e.taskOutputs[path.currentNode.TaskName()] = update.taskOutput

			// Remove completed path
			delete(activePaths, update.pathID)

			// Start any new paths
			for _, newPath := range update.newPaths {
				activePaths[newPath.id] = newPath
				go e.runPath(ctx, newPath, updates)
			}
		}
	}

	e.logger.Info(
		"workflow execution completed",
		"workflow_name", e.workflow.Name(),
		"total_usage", totalUsage,
	)
	return nil
}

func (e *Execution) runPath(ctx context.Context, path *executionPath, updates chan<- pathUpdate) {
	nextPathID := 0
	getNextPathID := func() string {
		nextPathID++
		return fmt.Sprintf("%s-%d", path.id, nextPathID)
	}

	logger := slogger.Ctx(ctx).
		With("path_id", path.id).
		With("execution_id", e.id)

	logger.Info("running path", "node", path.currentNode.Name())

	for {
		// Get agent for current task
		task := path.currentNode.Task()
		var agent dive.Agent
		if task.Agent() != nil {
			agent = task.Agent()
		} else {
			agent = e.environment.Agents()[0]
		}

		// Execute the task
		result, err := executeTask(ctx, task, agent)
		if err != nil {
			updates <- pathUpdate{pathID: path.id, err: err}
			return
		}

		nextEdges := path.currentNode.Next()
		var newPaths []*executionPath

		// Handle path branching
		if len(nextEdges) > 1 {
			// Create new paths for all edges
			for _, edge := range nextEdges {
				newPaths = append(newPaths, &executionPath{
					id:          getNextPathID(),
					currentNode: edge.To,
				})
			}
		} else if len(nextEdges) == 1 {
			// Continue on same path
			path.currentNode = nextEdges[0].To
			newPaths = []*executionPath{path}
		}

		// Send update
		updates <- pathUpdate{
			pathID:     path.id,
			taskOutput: result.Content,
			newPaths:   newPaths,
		}

		// If this path continues (single next edge), loop continues
		// Otherwise (no edges or multiple edges), this goroutine ends
		if len(nextEdges) != 1 {
			return
		}
	}
}

func executeTask(ctx context.Context, task dive.Task, agent dive.Agent) (*dive.TaskResult, error) {
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
