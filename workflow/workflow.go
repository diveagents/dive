package workflow

import (
	"context"
	"fmt"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/events"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
)

// Workflow represents a template for a repeatable process
type Workflow struct {
	name        string                         // Name of the workflow
	description string                         // Description of what the workflow does
	tasks       []dive.Task                    // Tasks that make up the workflow
	inputs      map[string]dive.WorkflowInput  // Expected input parameters
	outputs     map[string]dive.WorkflowOutput // Expected output parameters
	triggers    []Trigger                      // Events that can trigger this workflow
	metadata    map[string]string              // Additional metadata about the workflow
	agents      []dive.Agent                   // Agents available to execute tasks
	documents   document.Repository            // Document repository for the workflow
	logger      slogger.Logger                 // Logger for workflow operations
	taskOutputs map[string]string              // Map of task names to output file paths
}

// Input defines an expected input parameter for a workflow
type Input struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Default     interface{}
}

// Output defines an expected output parameter from a workflow
type Output struct {
	Name        string
	Type        string
	Description string
}

// Trigger defines an event that can start a workflow
type Trigger struct {
	Type        string                 // Type of trigger (schedule, event, etc)
	Config      map[string]interface{} // Configuration for the trigger
	Description string                 // Description of when/why this trigger fires
}

// WorkflowOptions configures a new workflow
type WorkflowOptions struct {
	Name         string
	Description  string
	Agents       []dive.Agent
	Repository   document.Repository
	Logger       slogger.Logger
	OutputDir    string
	OutputPlugin dive.OutputPlugin
	Inputs       map[string]dive.WorkflowInput
	Outputs      map[string]dive.WorkflowOutput
	Tasks        []dive.Task
}

// NewWorkflow creates a new workflow
func NewWorkflow(opts WorkflowOptions) (*Workflow, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("workflow name required")
	}
	if len(opts.Agents) == 0 {
		return nil, fmt.Errorf("at least one agent required")
	}
	if opts.Repository == nil {
		return nil, fmt.Errorf("document repository required")
	}
	if opts.Logger == nil {
		opts.Logger = slogger.NewDevNullLogger()
	}
	inputs := make(map[string]dive.WorkflowInput, len(opts.Inputs))
	for name, input := range opts.Inputs {
		inputs[name] = input
	}
	outputs := make(map[string]dive.WorkflowOutput, len(opts.Outputs))
	for name, output := range opts.Outputs {
		outputs[name] = output
	}
	tasks := make([]dive.Task, len(opts.Tasks))
	copy(tasks, opts.Tasks)
	return &Workflow{
		name:        opts.Name,
		description: opts.Description,
		agents:      opts.Agents,
		documents:   opts.Repository,
		logger:      opts.Logger,
		taskOutputs: make(map[string]string),
		inputs:      inputs,
		outputs:     outputs,
		metadata:    make(map[string]string),
		tasks:       tasks,
	}, nil
}

// Execute runs the workflow with the given inputs
func (w *Workflow) Execute(ctx context.Context, inputs map[string]interface{}) (events.Stream, error) {
	// Validate inputs against workflow.inputs requirements
	if err := w.validateInputs(inputs); err != nil {
		return nil, fmt.Errorf("invalid inputs: %w", err)
	}

	// Sort tasks into execution order
	orderedNames, err := orderTasks(w.tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to determine task execution order: %w", err)
	}

	// Convert ordered names back to tasks
	tasksByName := make(map[string]dive.Task, len(w.tasks))
	for _, task := range w.tasks {
		tasksByName[task.Name()] = task
	}
	var orderedTasks []dive.Task
	for _, name := range orderedNames {
		orderedTasks = append(orderedTasks, tasksByName[name])
	}

	// Create stream for workflow events
	stream := events.NewStream()

	// Run workflow execution in background
	go w.executeWorkflow(ctx, orderedTasks, stream)

	return stream, nil
}

func (w *Workflow) executeWorkflow(ctx context.Context, tasks []dive.Task, s events.Stream) {
	publisher := s.Publisher()
	defer publisher.Close()

	backgroundCtx := context.Background()
	totalUsage := llm.Usage{}

	w.logger.Debug("workflow execution started",
		"workflow_name", w.name,
		"task_count", len(tasks),
		"task_names", TaskNames(tasks),
	)

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
		result, err := w.executeTask(ctx, task, agent)
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
		w.taskOutputs[task.Name()] = result.Content

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

func (w *Workflow) executeTask(ctx context.Context, task dive.Task, agent dive.Agent) (*dive.TaskResult, error) {
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

func (w *Workflow) validateInputs(inputs map[string]interface{}) error {
	for name, input := range w.inputs {
		if input.Required {
			if _, exists := inputs[name]; !exists {
				return fmt.Errorf("required input %q missing", name)
			}
		}
	}
	return nil
}

func (w *Workflow) Name() string {
	return w.name
}

func (w *Workflow) Description() string {
	return w.description
}

func (w *Workflow) Inputs() map[string]dive.WorkflowInput {
	return w.inputs
}

func (w *Workflow) Outputs() map[string]dive.WorkflowOutput {
	return w.outputs
}

func (w *Workflow) Tasks() []dive.Task {
	tasks := make([]dive.Task, len(w.tasks))
	copy(tasks, w.tasks)
	return tasks
}

// Validate checks if the workflow is properly configured
func (w *Workflow) Validate() error {
	if w.name == "" {
		return fmt.Errorf("workflow name required")
	}
	if w.description == "" {
		return fmt.Errorf("workflow description required")
	}
	if len(w.tasks) == 0 {
		return fmt.Errorf("workflow must have at least one task")
	}
	// Validate task dependencies
	taskNames := make(map[string]bool)
	for _, task := range w.tasks {
		taskNames[task.Name()] = true
	}
	for _, task := range w.tasks {
		for _, dep := range task.Dependencies() {
			if !taskNames[dep] {
				return fmt.Errorf("task %q depends on unknown task %q", task.Name(), dep)
			}
		}
	}
	return nil
}
