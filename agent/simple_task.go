package agent

import "github.com/getstingrai/dive"

var _ dive.Task = &SimpleTask{}

type SimpleTask struct {
	name           string
	description    string
	expectedOutput string
	assignedAgent  dive.Agent
	dependencies   []string
	prompt         string
}

func (t *SimpleTask) Name() string {
	return t.name
}

func (t *SimpleTask) Description() string {
	return t.description
}

func (t *SimpleTask) ExpectedOutput() string {
	return t.expectedOutput
}

func (t *SimpleTask) AssignedAgent() dive.Agent {
	return t.assignedAgent
}

func (t *SimpleTask) Dependencies() []string {
	return t.dependencies
}

func (t *SimpleTask) Prompt(opts dive.TaskPromptOptions) string {
	return t.prompt
}

func (t *SimpleTask) Validate() error {
	return nil
}
