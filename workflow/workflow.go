package workflow

import (
	"fmt"

	"github.com/getstingrai/dive"
)

// Input defines an expected input parameter
type Input struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Default     interface{}
}

// Output defines an expected output parameter
type Output struct {
	Name        string
	Type        string
	Description string
}

// Workflow defines a repeatable process as a graph of tasks to be executed
type Workflow struct {
	name        string
	description string
	tasks       []dive.Task
	inputs      map[string]Input
	outputs     map[string]Output
	graph       *Graph
}

// WorkflowOptions configures a new workflow
type WorkflowOptions struct {
	Name        string
	Description string
	Tasks       []dive.Task
	Inputs      map[string]Input
	Outputs     map[string]Output
	Graph       *Graph
}

// NewWorkflow creates a new workflow
func NewWorkflow(opts WorkflowOptions) (*Workflow, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("workflow name required")
	}
	return &Workflow{
		name:        opts.Name,
		description: opts.Description,
		inputs:      opts.Inputs,
		outputs:     opts.Outputs,
		tasks:       opts.Tasks,
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

func (w *Workflow) Inputs() map[string]Input {
	return w.inputs
}

func (w *Workflow) Outputs() map[string]Output {
	return w.outputs
}

func (w *Workflow) Tasks() []dive.Task {
	return w.tasks
}

// Validate checks if the workflow is properly configured
func (w *Workflow) Validate() error {
	if w.name == "" {
		return fmt.Errorf("workflow name required")
	}
	if len(w.tasks) == 0 {
		return fmt.Errorf("workflow must have at least one task")
	}
	taskNames := make(map[string]bool, len(w.tasks))
	for _, task := range w.tasks {
		taskName := task.Name()
		if taskName == "" {
			return fmt.Errorf("task name required")
		}
		if _, exists := taskNames[taskName]; exists {
			return fmt.Errorf("duplicate task name %q", taskName)
		}
		taskNames[taskName] = true
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
