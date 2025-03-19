package workflow

import (
	"context"
	"fmt"

	"github.com/getstingrai/dive"
)

type Trigger struct {
	Name   string
	Type   string
	Config map[string]interface{}
}

// Workflow defines a repeatable process as a graph of tasks to be executed
type Workflow struct {
	name        string
	description string
	inputs      map[string]dive.Input
	outputs     map[string]dive.Output
	steps       []*Step
	graph       *Graph
	tasks       []dive.Task
	triggers    []*Trigger
}

// WorkflowOptions configures a new workflow
type WorkflowOptions struct {
	Name        string
	Description string
	Inputs      map[string]dive.Input
	Outputs     map[string]dive.Output
	Steps       []*Step
	Triggers    []*Trigger
}

// NewWorkflow creates and validates a Workflow
func NewWorkflow(opts WorkflowOptions) (*Workflow, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("workflow name required")
	}
	if len(opts.Steps) == 0 {
		return nil, fmt.Errorf("steps required")
	}
	graph := NewGraph(opts.Steps)
	if err := graph.Validate(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}
	for _, step := range opts.Steps {
		if err := step.Compile(context.Background()); err != nil {
			return nil, fmt.Errorf("step compilation failed: %w", err)
		}
	}
	w := &Workflow{
		name:        opts.Name,
		description: opts.Description,
		inputs:      opts.Inputs,
		outputs:     opts.Outputs,
		steps:       opts.Steps,
		graph:       graph,
		triggers:    opts.Triggers,
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

func (w *Workflow) Inputs() map[string]dive.Input {
	return w.inputs
}

func (w *Workflow) Outputs() map[string]dive.Output {
	return w.outputs
}

func (w *Workflow) Tasks() []dive.Task {
	return w.tasks
}

func (w *Workflow) Graph() *Graph {
	return w.graph
}

func (w *Workflow) Triggers() []*Trigger {
	return w.triggers
}

// Validate checks if the workflow is properly configured
func (w *Workflow) Validate() error {
	if w.name == "" {
		return fmt.Errorf("workflow name required")
	}
	if w.graph == nil {
		return fmt.Errorf("graph required")
	}
	startStep := w.graph.Start()
	if startStep == nil {
		return fmt.Errorf("graph start task required")
	}
	tasksMap := map[string]dive.Task{}
	for _, step := range w.graph.steps {
		tasksMap[step.Task().Name()] = step.Task()
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
