package workflow

import (
	"context"

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
