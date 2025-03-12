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
	inputs      map[string]Input
	outputs     map[string]Output
	graph       *Graph
	tasks       []dive.Task
}

// WorkflowOptions configures a new workflow
type WorkflowOptions struct {
	Name        string
	Description string
	Inputs      map[string]Input
	Outputs     map[string]Output
	Graph       *Graph
}

// NewWorkflow creates and validates a workflow
func NewWorkflow(opts WorkflowOptions) (*Workflow, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("workflow name required")
	}
	if opts.Graph == nil {
		return nil, fmt.Errorf("graph required")
	}
	if err := opts.Graph.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}
	w := &Workflow{
		name:        opts.Name,
		description: opts.Description,
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
	// taskNames := make(map[string]bool, len(w.tasks))
	// for _, task := range w.tasks {
	// 	taskName := task.Name()
	// 	if taskName == "" {
	// 		return fmt.Errorf("task name required")
	// 	}
	// 	if _, exists := taskNames[taskName]; exists {
	// 		return fmt.Errorf("duplicate task name %q", taskName)
	// 	}
	// 	taskNames[taskName] = true
	// }
	if w.graph == nil {
		return fmt.Errorf("graph required")
	}
	startNode := w.graph.Start()
	if startNode == nil {
		return fmt.Errorf("graph start task required")
	}
	tasksMap := map[string]dive.Task{}
	for _, node := range w.graph.nodes {
		tasksMap[node.Task().Name()] = node.Task()
	}
	for _, nodeName := range w.graph.Names() {
		node, ok := w.graph.Get(nodeName)
		if !ok {
			return fmt.Errorf("task %q not found (1)", nodeName)
		}
		for _, edge := range node.Next() {
			targetNode, ok := w.graph.Get(edge.To)
			if !ok {
				return fmt.Errorf("task %q not found (2)", edge.To)
			}
			if targetNode.TaskName() == "" {
				return fmt.Errorf("task %q has no name", edge.To)
			}
			if _, found := tasksMap[targetNode.TaskName()]; !found {
				return fmt.Errorf("task %q not found (3)", targetNode.TaskName())
			}
		}
	}
	for _, task := range tasksMap {
		w.tasks = append(w.tasks, task)
	}
	return nil
}
