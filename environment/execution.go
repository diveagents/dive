package environment

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
)

// PathStatus represents the current state of an execution path
type PathStatus string

const (
	PathStatusPending   PathStatus = "pending"
	PathStatusRunning   PathStatus = "running"
	PathStatusCompleted PathStatus = "completed"
	PathStatusFailed    PathStatus = "failed"
)

// PathState tracks the state of an execution path
type PathState struct {
	ID          string
	Status      PathStatus
	CurrentStep *workflow.Step
	StartTime   time.Time
	EndTime     time.Time
	Error       error
	StepOutputs map[string]string
}

// Copy returns a shallow copy of the path state.
func (p *PathState) Copy() *PathState {
	return &PathState{
		ID:          p.ID,
		Status:      p.Status,
		CurrentStep: p.CurrentStep,
		StartTime:   p.StartTime,
		EndTime:     p.EndTime,
		Error:       p.Error,
		StepOutputs: p.StepOutputs,
	}
}

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

type executionPath struct {
	id          string
	currentStep *workflow.Step
}

type pathUpdate struct {
	pathID     string
	stepName   string
	stepOutput string
	newPaths   []*executionPath
	err        error
	isDone     bool // true if this path should be removed from active paths
}

// Execution represents a single run of a workflow
type Execution struct {
	id          string                 // Unique identifier for this execution
	environment *Environment           // Environment that the execution belongs to
	workflow    *workflow.Workflow     // Workflow being executed
	status      Status                 // Current status of the execution
	startTime   time.Time              // When the execution started
	endTime     time.Time              // When the execution completed (or failed/canceled)
	inputs      map[string]interface{} // Input parameters for the workflow
	outputs     map[string]interface{} // Output values from the workflow
	err         error                  // Error if execution failed
	parentID    string                 // ID of parent execution (for sub-executions)
	childIDs    []string               // IDs of child executions
	metadata    map[string]string      // Additional metadata about the execution
	logger      slogger.Logger         // Logger for the execution
	paths       map[string]*PathState  // Track all paths by ID
	mutex       sync.RWMutex           // Mutex for thread-safe operations
	doneWg      sync.WaitGroup         // Wait group for the execution
}

// ExecutionOptions are the options for creating a new execution.
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

// NewExecution creates a new execution of a workflow. Begin the execution by
// calling execution.Run().
func NewExecution(opts ExecutionOptions) *Execution {
	if opts.Status == "" {
		opts.Status = StatusPending
	}
	if opts.Logger == nil {
		opts.Logger = slogger.NewDevNullLogger()
	}
	execution := &Execution{
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
		paths:       make(map[string]*PathState),
		mutex:       sync.RWMutex{},
		doneWg:      sync.WaitGroup{},
	}
	execution.doneWg.Add(1)
	return execution
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
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.status
}

func (e *Execution) StartTime() time.Time {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.startTime
}

func (e *Execution) EndTime() time.Time {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.endTime
}

func (e *Execution) Wait() error {
	e.doneWg.Wait()
	return e.err
}

// Run the execution until completion in a blocking manner.
func (e *Execution) Run(ctx context.Context) error {
	e.mutex.Lock()
	if e.status != StatusPending {
		e.mutex.Unlock()
		return fmt.Errorf("execution can only be run from pending state, current status: %s", e.status)
	}
	e.status = StatusRunning
	e.startTime = time.Now()
	e.mutex.Unlock()

	defer e.doneWg.Done() // Ensure we always mark as done when execution completes

	err := e.run(ctx)

	e.mutex.Lock()
	e.endTime = time.Now()
	if err != nil {
		e.logger.Error("workflow execution failed", "error", err)
		e.status = StatusFailed
		e.err = err
	} else {
		e.logger.Info("workflow execution completed", "execution_id", e.id)
		e.status = StatusCompleted
		e.err = nil
	}
	e.mutex.Unlock()

	return err
}

func (e *Execution) run(ctx context.Context) error {
	graph := e.workflow.Graph()
	totalUsage := llm.Usage{}

	e.logger.Info(
		"workflow execution started",
		"workflow_name", e.workflow.Name(),
		"start_step", graph.Start()[0].Name(),
	)

	// Channel for path updates
	updates := make(chan pathUpdate)
	activePaths := make(map[string]*executionPath)

	// Start initial paths
	for i, startStep := range graph.Start() {
		initialPath := &executionPath{
			id:          fmt.Sprintf("path-%d", i+1),
			currentStep: startStep,
		}
		activePaths[initialPath.id] = initialPath
		e.addPath(initialPath)
		go e.runPath(ctx, graph, initialPath, updates)
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
				e.updatePathState(update.pathID, func(state *PathState) {
					state.Status = PathStatusFailed
					state.Error = update.err
					state.EndTime = time.Now()
				})
				return update.err
			}

			// Store task output and update path state
			e.updatePathState(update.pathID, func(state *PathState) {
				state.StepOutputs[update.stepName] = update.stepOutput
				if update.isDone {
					state.Status = PathStatusCompleted
					state.EndTime = time.Now()
				}
			})

			// Remove path if it's done
			if update.isDone {
				delete(activePaths, update.pathID)
			}

			// Start any new paths
			for _, newPath := range update.newPaths {
				activePaths[newPath.id] = newPath
				e.addPath(newPath)
				go e.runPath(ctx, graph, newPath, updates)
			}
		}
	}

	// Check if any paths failed
	e.mutex.RLock()
	var failedPaths []string
	for _, state := range e.paths {
		if state.Status == PathStatusFailed {
			failedPaths = append(failedPaths, state.ID)
		}
	}
	e.mutex.RUnlock()

	if len(failedPaths) > 0 {
		return fmt.Errorf("execution completed with failed paths: %v", failedPaths)
	}

	e.logger.Info(
		"workflow execution completed",
		"workflow_name", e.workflow.Name(),
		"total_usage", totalUsage,
	)
	return nil
}

// Runs a single execution path in its own goroutine. Returns when the path
// completes, fails, or splits into multiple new paths.
func (e *Execution) runPath(ctx context.Context, graph *workflow.Graph, path *executionPath, updates chan<- pathUpdate) {
	nextPathID := 0
	getNextPathID := func() string {
		nextPathID++
		return fmt.Sprintf("%s-%d", path.id, nextPathID)
	}

	logger := slogger.Ctx(ctx).
		With("path_id", path.id).
		With("execution_id", e.id)

	logger.Info("running path", "step", path.currentStep.Name())

	// Update path state to running
	e.updatePathState(path.id, func(state *PathState) {
		state.Status = PathStatusRunning
		state.StartTime = time.Now()
	})

	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("path %s panicked: %v", path.id, r)
			e.updatePathState(path.id, func(state *PathState) {
				state.Status = PathStatusFailed
				state.Error = err
				state.EndTime = time.Now()
			})
			updates <- pathUpdate{pathID: path.id, err: err}
		}
	}()

	for {
		// Get agent for current task
		currentStep := path.currentStep
		task := currentStep.Task()
		var agent dive.Agent
		if task.Agent() != nil {
			agent = task.Agent()
		} else {
			agent = e.environment.Agents()[0]
		}
		withInputs := currentStep.With()
		taskInputs := task.Inputs()

		// Determine task inputs
		processedInputs := make(map[string]interface{})
		for name, input := range taskInputs {
			value, exists := withInputs[name]
			if !exists {
				if input.Default == nil {
					err := fmt.Errorf("required input %q not provided in step %q", name, currentStep.Name())
					e.logger.Error("task execution failed", "error", err)
					updates <- pathUpdate{pathID: path.id, err: err}
					return
				}
				value = input.Default
			}
			if code, ok := currentStep.Code(name); ok {
				result, err := eval(ctx, code, map[string]any{
					"inputs": e.inputs,
				})
				if err != nil {
					e.logger.Error("task execution failed", "error", err)
					updates <- pathUpdate{pathID: path.id, err: err}
					return
				}
				processedInputs[name] = result
			} else {
				processedInputs[name] = value
			}
		}

		// Confirm there were no invalid "with" parameters provided
		for name := range withInputs {
			if _, exists := taskInputs[name]; !exists {
				err := fmt.Errorf("unknown input %q provided in step %q", name, currentStep.Name())
				e.logger.Error("task execution failed", "error", err)
				updates <- pathUpdate{pathID: path.id, err: err}
				return
			}
		}

		// Update path state with current task
		e.updatePathState(path.id, func(state *PathState) {
			state.CurrentStep = currentStep
		})

		result, err := executeTask(ctx, agent, task, processedInputs)
		if err != nil {
			logger.Error("task execution failed",
				"task", task.Name(),
				"error", err,
			)
			e.updatePathState(path.id, func(state *PathState) {
				state.Status = PathStatusFailed
				state.Error = err
				state.EndTime = time.Now()
			})
			updates <- pathUpdate{pathID: path.id, err: err}
			return
		}

		nextEdges := currentStep.Next()
		var newPaths []*executionPath
		var pathError error

		// Handle path branching
		if len(nextEdges) > 1 {
			// Create new paths for all edges
			for _, edge := range nextEdges {
				nextStep, ok := graph.Get(edge.To)
				if !ok {
					pathError = fmt.Errorf("next step not found: %s", edge.To)
					logger.Error("next step not found", "step", edge.To)
					continue
				}
				newPaths = append(newPaths, &executionPath{
					id:          getNextPathID(),
					currentStep: nextStep,
				})
			}
		} else if len(nextEdges) == 1 {
			// Continue on same path
			nextStep, ok := graph.Get(nextEdges[0].To)
			if !ok {
				pathError = fmt.Errorf("next step not found: %s", nextEdges[0].To)
				logger.Error("next step not found", "step", nextEdges[0].To)
			} else {
				updates <- pathUpdate{
					pathID:     path.id,
					stepName:   currentStep.Name(),
					stepOutput: result.Content,
					newPaths:   nil,
					isDone:     false, // Explicitly indicate path continues
				}
				path.currentStep = nextStep
				continue
			}
		}

		if pathError != nil {
			e.updatePathState(path.id, func(state *PathState) {
				state.Status = PathStatusFailed
				state.Error = pathError
				state.EndTime = time.Now()
			})
			updates <- pathUpdate{pathID: path.id, err: pathError}
			return
		}

		// Send final update for this path
		updates <- pathUpdate{
			pathID:     path.id,
			stepName:   currentStep.Name(),
			stepOutput: result.Content,
			newPaths:   newPaths,
			isDone:     true,
		}

		// If this path continues (single next edge), loop continues
		// Otherwise (no edges or multiple edges), this goroutine ends
		if len(nextEdges) != 1 {
			e.updatePathState(path.id, func(state *PathState) {
				state.Status = PathStatusCompleted
				state.EndTime = time.Now()
			})
			return
		}
	}
}

// updatePathState updates the state of a path
func (e *Execution) updatePathState(pathID string, updateFn func(*PathState)) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if state, exists := e.paths[pathID]; exists {
		updateFn(state)
	}
}

// addPath adds a new path to the execution
func (e *Execution) addPath(path *executionPath) *PathState {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	state := &PathState{
		ID:          path.id,
		Status:      PathStatusPending,
		CurrentStep: path.currentStep,
		StartTime:   time.Now(),
		StepOutputs: make(map[string]string),
	}
	e.paths[path.id] = state
	return state
}

// GetError returns the top-level execution error, if any.
func (e *Execution) GetError() error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	return e.err
}

// PathStates returns a copy of all path states in the execution.
func (e *Execution) PathStates() []*PathState {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	paths := make([]*PathState, 0, len(e.paths))
	for _, state := range e.paths {
		paths = append(paths, state.Copy())
	}
	return paths
}

// ExecutionStats provides statistics about the execution
type ExecutionStats struct {
	TotalPaths     int           `json:"total_paths"`
	ActivePaths    int           `json:"active_paths"`
	CompletedPaths int           `json:"completed_paths"`
	FailedPaths    int           `json:"failed_paths"`
	StartTime      time.Time     `json:"start_time"`
	EndTime        time.Time     `json:"end_time"`
	Duration       time.Duration `json:"duration"`
}

// GetStats returns current execution statistics
func (e *Execution) GetStats() ExecutionStats {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	stats := ExecutionStats{
		TotalPaths: len(e.paths),
		StartTime:  e.startTime,
		EndTime:    e.endTime,
	}

	for _, state := range e.paths {
		switch state.Status {
		case PathStatusRunning:
			stats.ActivePaths++
		case PathStatusCompleted:
			stats.CompletedPaths++
		case PathStatusFailed:
			stats.FailedPaths++
		}
	}

	if !e.endTime.IsZero() {
		stats.Duration = e.endTime.Sub(e.startTime)
	} else if !e.startTime.IsZero() {
		stats.Duration = time.Since(e.startTime)
	}

	return stats
}

func (e *Execution) StepOutputs() map[string]string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	outputs := make(map[string]string)
	for _, pathState := range e.paths {
		for stepName, output := range pathState.StepOutputs {
			outputs[stepName] = output
		}
	}
	return outputs
}

func executeTask(ctx context.Context, agent dive.Agent, task dive.Task, inputs map[string]interface{}) (*dive.TaskResult, error) {
	iterator, err := agent.Work(ctx, task, inputs)
	if err != nil {
		return nil, fmt.Errorf("failed to start task %q: %w", task.Name(), err)
	}
	defer iterator.Close()

	taskResult, err := dive.WaitForEvent[*dive.TaskResult](ctx, iterator)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for task result: %w", err)
	}
	return taskResult, nil
}

func eval(ctx context.Context, code *compiler.Code, inputs map[string]any) (any, error) {
	result, err := risor.EvalCode(ctx, code, risor.WithGlobals(map[string]any{
		"inputs": inputs,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate code: %w", err)
	}
	switch result := result.(type) {
	case *object.String:
		return result.Value(), nil
	case *object.Int:
		return result.Value(), nil
	case *object.Float:
		return result.Value(), nil
	case *object.Bool:
		return result.Value(), nil
	default:
		return nil, fmt.Errorf("unsupported result type: %T", result)
	}
}
