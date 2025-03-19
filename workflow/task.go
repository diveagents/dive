package workflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/eval"
)

var _ dive.Task = &Task{}

// TaskOptions are used to configure a new Task
type TaskOptions struct {
	Name        string
	Description string
	Kind        string
	Inputs      map[string]dive.Input
	Outputs     map[string]dive.Output
	Timeout     time.Duration
	Agent       dive.Agent
}

// Task represents one unit of work within a Workflow. It may be reused by being
// called multiple times.
type Task struct {
	name         string
	description  string
	kind         string
	inputs       map[string]dive.Input
	outputs      map[string]dive.Output
	timeout      time.Duration
	agent        dive.Agent
	nameIsRandom bool
	descTemplate *eval.String
	descErr      error
}

// NewTask returns a new Task that is configured with the given options.
func NewTask(opts TaskOptions) *Task {
	var nameIsRandom bool
	if opts.Name == "" {
		opts.Name = fmt.Sprintf("task-%s", petname.Generate(2, "-"))
		nameIsRandom = true
	}
	var descErr error
	var descTemplate *eval.String
	descTemplate, descErr = eval.NewString(opts.Description, map[string]any{
		"inputs": nil,
	})
	return &Task{
		name:         opts.Name,
		description:  opts.Description,
		kind:         opts.Kind,
		inputs:       opts.Inputs,
		outputs:      opts.Outputs,
		timeout:      opts.Timeout,
		agent:        opts.Agent,
		nameIsRandom: nameIsRandom,
		descTemplate: descTemplate,
		descErr:      descErr,
	}
}

func (t *Task) Name() string                    { return t.name }
func (t *Task) Description() string             { return t.description }
func (t *Task) Kind() string                    { return t.kind }
func (t *Task) Inputs() map[string]dive.Input   { return t.inputs }
func (t *Task) Outputs() map[string]dive.Output { return t.outputs }
func (t *Task) Timeout() time.Duration          { return t.timeout }
func (t *Task) Agent() dive.Agent               { return t.agent }

// Validate checks if the task is properly configured
func (t *Task) Validate() error {
	if t.name == "" {
		return fmt.Errorf("task name required")
	}
	if t.description == "" {
		return fmt.Errorf("description required for task %q", t.name)
	}
	return nil
}

// Prompt returns the LLM prompt for the task
func (t *Task) Prompt(opts dive.TaskPromptOptions) (string, error) {
	if t.descErr != nil {
		return "", t.descErr
	}

	// Resolve inputs
	processedInputs := make(map[string]any)
	for name, input := range t.inputs {
		if value, ok := opts.Inputs[name]; ok {
			processedInputs[name] = value
		} else if input.Default != nil {
			processedInputs[name] = input.Default
		} else {
			return "", fmt.Errorf("input %q is required", name)
		}
	}

	var intro string
	if t.name != "" && !t.nameIsRandom {
		intro = fmt.Sprintf("Let's work on a new task named %q.", t.name)
	} else {
		intro = "Let's work on a new task."
	}

	lines := []string{}

	if t.description != "" {
		description, err := t.descTemplate.Eval(context.Background(), map[string]any{
			"inputs": processedInputs,
		})
		if err != nil {
			return "", err
		}
		lines = append(lines, description)
	}

	if len(t.outputs) > 0 {
		outputLines := []string{"Please provide the following outputs:"}
		for name, output := range t.outputs {
			outputLines = append(outputLines, fmt.Sprintf("- %s (%s): %s", name, output.Type, output.Description))
			if output.Format != "" {
				outputLines = append(outputLines, fmt.Sprintf("  Format: %s", output.Format))
			}
		}
		lines = append(lines, strings.Join(outputLines, "\n"))
	}

	promptParts := []string{
		formatBlock(intro, "task", strings.Join(lines, "\n\n")),
	}

	var contextParts []string
	if opts.Context != "" {
		contextParts = append(contextParts, opts.Context)
	}
	if len(contextParts) > 0 {
		contextHeading := "Use this context while working on the task:"
		promptParts = append(promptParts, formatBlock(contextHeading, "context", strings.Join(contextParts, "\n\n")))
	}

	promptParts = append(promptParts, "\n\nPlease begin working on the task.")
	return strings.Join(promptParts, "\n\n"), nil
}

func formatBlock(heading, blockType, content string) string {
	return fmt.Sprintf("%s\n\n```%s\n%s\n```", heading, blockType, content)
}
