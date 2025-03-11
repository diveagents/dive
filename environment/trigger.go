package environment

import (
	"context"

	"github.com/getstingrai/dive/workflow"
)

type Trigger struct {
	name      string
	workflows []*workflow.Workflow
	env       *Environment
}

func NewTrigger(name string, env *Environment) *Trigger {
	return &Trigger{
		name: name,
		env:  env,
	}
}

func (t *Trigger) Name() string {
	return t.name
}

func (t *Trigger) Subscribe(workflow *workflow.Workflow) error {
	t.workflows = append(t.workflows, workflow)
	return nil
}

func (t *Trigger) Unsubscribe(workflow *workflow.Workflow) error {
	for i, w := range t.workflows {
		if w == workflow {
			t.workflows = append(t.workflows[:i], t.workflows[i+1:]...)
		}
	}
	return nil
}

func (t *Trigger) Fire(ctx context.Context, input map[string]interface{}) error {
	for _, w := range t.workflows {
		if err := t.env.StartWorkflow(ctx, w); err != nil {
			return err
		}
	}
	return nil
}
