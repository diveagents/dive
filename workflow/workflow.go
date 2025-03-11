package workflow

import (
	"fmt"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/slogger"
)

// Workflow represents a template for a repeatable process
type Workflow struct {
	name        string
	description string
	agents      []dive.Agent
	tasks       []dive.Task
	inputs      map[string]dive.WorkflowInput
	outputs     map[string]dive.WorkflowOutput
	taskOutputs map[string]string
	graph       *Graph
}

// WorkflowOptions configures a new workflow
type WorkflowOptions struct {
	Name        string
	Description string
	Agents      []dive.Agent
	Tasks       []dive.Task
	Inputs      map[string]dive.WorkflowInput
	Outputs     map[string]dive.WorkflowOutput
	Graph       *Graph
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
		repository:  opts.Repository,
		logger:      opts.Logger,
		inputs:      inputs,
		outputs:     outputs,
		tasks:       tasks,
		taskOutputs: map[string]string{},
	}, nil
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
