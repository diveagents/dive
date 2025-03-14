package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/getstingrai/dive"
)

var _ dive.Task = &Task{}

// TaskOptions are used to create a Task
type TaskOptions struct {
	Name        string
	Description string
	Kind        string
	Inputs      map[string]dive.Input
	Outputs     map[string]dive.Output
	Timeout     time.Duration
	Agent       dive.Agent
}

// Task represents one unit of work in a workflow
type Task struct {
	name         string
	description  string
	kind         string
	inputs       map[string]dive.Input
	outputs      map[string]dive.Output
	timeout      time.Duration
	agent        dive.Agent
	nameIsRandom bool
}

// NewTask creates a new Task from a TaskOptions
func NewTask(opts TaskOptions) *Task {
	var nameIsRandom bool
	if opts.Name == "" {
		opts.Name = fmt.Sprintf("task-%s", petname.Generate(2, "-"))
		nameIsRandom = true
	}
	fmt.Printf("inputs: %+v\n", opts.Inputs)
	fmt.Printf("outputs: %+v\n", opts.Outputs)
	return &Task{
		name:         opts.Name,
		description:  opts.Description,
		kind:         opts.Kind,
		inputs:       opts.Inputs,
		outputs:      opts.Outputs,
		timeout:      opts.Timeout,
		agent:        opts.Agent,
		nameIsRandom: nameIsRandom,
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
func (t *Task) Prompt(opts dive.TaskPromptOptions) string {
	var intro string
	if t.name != "" && !t.nameIsRandom {
		intro = fmt.Sprintf("Let's work on a new task named %q.", t.name)
	} else {
		intro = "Let's work on a new task."
	}
	lines := []string{}
	if t.description != "" {
		lines = append(lines, t.description)
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
	return strings.Join(promptParts, "\n\n")
}

var validFilenamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func isSimpleFilename(filename string) bool {
	// Check if filename is empty or too long
	if filename == "" || len(filename) > 255 {
		return false
	}
	// Only allow alphanumeric characters, dash, underscore, and period
	// Must start with an alphanumeric character
	return validFilenamePattern.MatchString(filename)
}

func formatBlock(heading, blockType, content string) string {
	return fmt.Sprintf("%s\n\n```%s\n%s\n```", heading, blockType, content)
}
