package agent

import (
	"strings"
	"time"

	"github.com/getstingrai/dive"
)

var _ dive.Task = &SimpleTask{}

type SimpleTask struct {
	name          string
	description   string
	inputs        map[string]*dive.Input
	output        *dive.Output
	assignedAgent dive.Agent
	dependencies  []string
	prompt        string
	timeout       time.Duration
}

func (t *SimpleTask) Name() string {
	return t.name
}

func (t *SimpleTask) Description() string {
	return t.description
}

func (t *SimpleTask) Timeout() time.Duration {
	return t.timeout
}

func (t *SimpleTask) Inputs() map[string]*dive.Input {
	return t.inputs
}

func (t *SimpleTask) Output() *dive.Output {
	return t.output
}

func (t *SimpleTask) Agent() dive.Agent {
	return t.assignedAgent
}

func (t *SimpleTask) Dependencies() []string {
	return t.dependencies
}

func (t *SimpleTask) Prompt(opts dive.TaskPromptOptions) (string, error) {
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
	if t.output != nil {
		prompt += "\n\nExpected Outputs:"
		prompt += "\n- " + t.output.Name
		if t.output.Description != "" {
			prompt += ": " + t.output.Description
		}
		if t.output.Format != "" {
			prompt += "\n  Format: " + t.output.Format
		}
	}

	return prompt, nil
}

func (t *SimpleTask) Validate() error {
	return nil
}
