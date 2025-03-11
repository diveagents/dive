package agent

import (
	"strings"

	"github.com/getstingrai/dive"
)

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
	prompt := "Complete the following task:\n\n" + t.description
	var contextParts []string
	if opts.Context != "" {
		contextParts = append(contextParts, opts.Context)
	}
	if len(t.dependencies) > 0 {
		contextParts = append(contextParts, strings.Join(t.dependencies, ", "))
	}
	if len(contextParts) > 0 {
		prompt += "\n\nContext:\n" + strings.Join(contextParts, "\n")
	}
	if t.expectedOutput != "" {
		prompt += "\n\nExpected output:\n" + t.expectedOutput
	}
	return prompt
}

func (t *SimpleTask) Validate() error {
	return nil
}
