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

// NewWorkflow creates and validates a workflow
func NewWorkflow(opts WorkflowOptions) (*Workflow, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("workflow name required")
	}
	w := &Workflow{
		name:        opts.Name,
		description: opts.Description,
		tasks:       opts.Tasks,
		inputs:      opts.Inputs,
		outputs:     opts.Outputs,
		graph:       opts.Graph,
	}
	if err := w.Validate(); err != nil {
		return nil, err
	}
	return w, nil
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

func (w *Workflow) Graph() *Graph {
	return w.graph
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
	if w.graph == nil {
		return fmt.Errorf("graph required")
	}
	startNode := w.graph.StartNode()
	if startNode == nil {
		return fmt.Errorf("graph start task required")
	}
	for _, nodeName := range w.graph.NodeNames() {
		node, ok := w.graph.GetNode(nodeName)
		if !ok {
			return fmt.Errorf("task %q not found", nodeName)
		}
		if _, exists := taskNames[nodeName]; !exists {
			return fmt.Errorf("task %q not found", nodeName)
		}
		for _, edge := range node.Next {
			if _, exists := taskNames[edge.To.Name()]; !exists {
				return fmt.Errorf("task %q not found", edge.To.Name())
			}
		}
	}
	return nil
}
