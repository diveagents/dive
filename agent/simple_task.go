package agent

import (
	"strings"

	"github.com/getstingrai/dive"
)

var _ dive.Task = &SimpleTask{}

type SimpleTask struct {
	name          string
	description   string
	inputs        map[string]dive.Input
	outputs       map[string]dive.Output
	assignedAgent dive.Agent
	dependencies  []string
	prompt        string
}

func (t *SimpleTask) Name() string {
	return t.name
}

func (t *SimpleTask) Description() string {
	return t.description
}

func (t *SimpleTask) Inputs() map[string]dive.Input {
	return t.inputs
}

func (t *SimpleTask) Outputs() map[string]dive.Output {
	return t.outputs
}

func (t *SimpleTask) Agent() dive.Agent {
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

	// Add output descriptions if any exist
	if len(t.outputs) > 0 {
		prompt += "\n\nExpected Outputs:"
		for name, output := range t.outputs {
			prompt += "\n- " + name
			if output.Description != "" {
				prompt += ": " + output.Description
			}
			if output.Format != "" {
				prompt += "\n  Format: " + output.Format
			}
		}
	}

	return prompt
}

func (t *SimpleTask) Validate() error {
	return nil
}
