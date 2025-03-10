package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/events"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
)

// Workflow represents a template for a repeatable process
type Workflow struct {
	name         string                         // Name of the workflow
	description  string                         // Description of what the workflow does
	tasks        []*Task                        // Tasks that make up the workflow
	inputs       map[string]dive.WorkflowInput  // Expected input parameters
	outputs      map[string]dive.WorkflowOutput // Expected output parameters
	triggers     []Trigger                      // Events that can trigger this workflow
	metadata     map[string]string              // Additional metadata about the workflow
	agents       []dive.Agent                   // Agents available to execute tasks
	documents    document.Repository            // Document repository for the workflow
	logger       slogger.Logger                 // Logger for workflow operations
	outputDir    string                         // Directory for workflow outputs
	outputPlugin dive.OutputPlugin              // Plugin for handling task outputs
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

	return &Workflow{
		name:         opts.Name,
		description:  opts.Description,
		agents:       opts.Agents,
		documents:    opts.Repository,
		logger:       opts.Logger,
		outputDir:    opts.OutputDir,
		outputPlugin: opts.OutputPlugin,
		inputs:       make(map[string]dive.WorkflowInput),
		outputs:      make(map[string]dive.WorkflowOutput),
		metadata:     make(map[string]string),
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
	tasksByName := make(map[string]*Task)
	for _, task := range w.tasks {
		tasksByName[task.Name()] = task
	}
	var orderedTasks []*Task
	for _, name := range orderedNames {
		orderedTasks = append(orderedTasks, tasksByName[name])
	}

	// Create stream for workflow events
	stream := events.NewStream()

	// Run workflow execution in background
	go w.executeWorkflow(ctx, orderedTasks, inputs, stream)

	return stream, nil
}

func (w *Workflow) executeWorkflow(ctx context.Context, tasks []*Task, inputs map[string]interface{}, s events.Stream) {
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

		// Check if task output already exists
		done, err := w.outputPlugin.OutputExists(ctx, task.Name(), "")
		if err != nil {
			w.logger.Error("failed to check if task output exists", "error", err)
		}

		if !done {
			// Execute the task
			result, err := w.executeTask(ctx, task, agent, inputs, publisher)
			if err != nil {
				publisher.Send(backgroundCtx, &events.Event{
					Type:      "workflow.error",
					TaskName:  task.Name(),
					AgentName: agent.Name(),
					Error:     err.Error(),
				})
				return
			}

			// Store task results
			if err := w.outputPlugin.WriteOutput(ctx, task.Name(), "", result.Content); err != nil {
				publisher.Send(backgroundCtx, &events.Event{
					Type:      "workflow.error",
					TaskName:  task.Name(),
					AgentName: agent.Name(),
					Error:     err.Error(),
				})
				return
			}

			// totalUsage.Add(result.Usage)
		}
	}

	w.logger.Info("workflow execution completed",
		"workflow_name", w.name,
		"total_usage", totalUsage,
	)

	publisher.Send(backgroundCtx, &events.Event{Type: "workflow.done"})
}

func (w *Workflow) executeTask(
	ctx context.Context,
	task *Task,
	agent dive.Agent,
	inputs map[string]interface{},
	publisher events.Publisher,
) (*dive.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (w *Workflow) AddTask(task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if err := task.Validate(); err != nil {
		return fmt.Errorf("invalid task: %w", err)
	}
	w.tasks = append(w.tasks, task)
	return nil
}

func (w *Workflow) AddInput(input dive.WorkflowInput) error {
	if input.Name == "" {
		return fmt.Errorf("input name required")
	}
	w.inputs[input.Name] = input
	return nil
}

func (w *Workflow) AddOutput(output dive.WorkflowOutput) error {
	if output.Name == "" {
		return fmt.Errorf("output name required")
	}
	w.outputs[output.Name] = output
	return nil
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
	for i, task := range w.tasks {
		tasks[i] = task
	}
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
