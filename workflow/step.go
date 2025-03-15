package workflow

import (
	"context"
	"fmt"

	"github.com/getstingrai/dive"
)

type Condition interface {
	Evaluate(ctx context.Context, inputs map[string]interface{}) (bool, error)
}

type Variable interface {
	GetValue() any
	SetValue(value any)
}

type Edge struct {
	To        string
	Condition Condition
}

type EachBlock struct {
	Array         string
	As            string
	Parallel      bool
	MaxConcurrent int
}

type Step struct {
	name    string
	task    dive.Task
	inputs  map[string]interface{}
	outputs map[string]interface{}
	next    []*Edge
	isStart bool
	each    *EachBlock
}

type StepOptions struct {
	Name    string
	Task    dive.Task
	Inputs  map[string]interface{}
	Outputs map[string]interface{}
	Next    []*Edge
	IsStart bool
	Each    *EachBlock
}

func NewStep(opts StepOptions) *Step {
	return &Step{
		name:    opts.Name,
		task:    opts.Task,
		inputs:  opts.Inputs,
		outputs: opts.Outputs,
		next:    opts.Next,
		isStart: opts.IsStart,
		each:    opts.Each,
	}
}

func (n *Step) IsStart() bool {
	return n.isStart
}

func (n *Step) SetIsStart(isStart bool) {
	n.isStart = isStart
}

func (n *Step) Name() string {
	return n.name
}

func (n *Step) Task() dive.Task {
	return n.task
}

func (n *Step) TaskName() string {
	return n.task.Name()
}

func (n *Step) Inputs() map[string]interface{} {
	return n.inputs
}

func (n *Step) Outputs() map[string]interface{} {
	return n.outputs
}

func (n *Step) Next() []*Edge {
	return n.next
}

func (n *Step) Each() *EachBlock {
	return n.each
}

type Graph struct {
	steps map[string]*Step
	start []*Step
}

type GraphOptions struct {
	Nodes map[string]*Step
}

// NewGraph creates a new graph containing the given steps
func NewGraph(steps []*Step) *Graph {
	if len(steps) == 1 {
		for _, step := range steps {
			step.isStart = true
			break
		}
	}
	var startNodes []*Step
	graphNodes := make(map[string]*Step, len(steps))
	for _, step := range steps {
		if step.name == "" {
			step.name = step.TaskName()
		}
		if step.isStart {
			startNodes = append(startNodes, step)
		}
		graphNodes[step.name] = step
	}
	return &Graph{
		steps: graphNodes,
		start: startNodes,
	}
}

// Start returns the start step(s) of the graph
func (g *Graph) Start() []*Step {
	return g.start
}

// Get returns a step by name
func (g *Graph) Get(name string) (*Step, bool) {
	step, ok := g.steps[name]
	return step, ok
}

// Names returns the names of all steps in the graph
func (g *Graph) Names() []string {
	names := make([]string, 0, len(g.steps))
	for name := range g.steps {
		names = append(names, name)
	}
	return names
}

func (g *Graph) Validate() error {
	if len(g.steps) == 0 {
		return fmt.Errorf("graph must have at least one step")
	}
	for _, step := range g.steps {
		if step.name == "" {
			return fmt.Errorf("step name cannot be empty")
		}
		for _, edge := range step.next {
			if _, ok := g.steps[edge.To]; !ok {
				return fmt.Errorf("edge to step %q not found", edge.To)
			}
		}
	}
	return nil
}
