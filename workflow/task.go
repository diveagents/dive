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
	Name           string
	Description    string
	Kind           string
	Inputs         map[string]Input
	Outputs        map[string]Output
	Timeout        time.Duration
	Agent          dive.Agent
	ExpectedOutput string
	OutputFormat   dive.OutputFormat
	OutputFile     string
	OutputObject   interface{}
}

// Task represents one unit of work in a workflow
type Task struct {
	name           string
	description    string
	kind           string
	inputs         map[string]Input
	outputs        map[string]Output
	timeout        time.Duration
	agent          dive.Agent
	expectedOutput string
	outputFormat   dive.OutputFormat
	outputFile     string
	outputObject   interface{}
	nameIsRandom   bool
}

// NewTask creates a new Task from a TaskOptions
func NewTask(opts TaskOptions) *Task {
	var nameIsRandom bool
	if opts.Name == "" {
		opts.Name = fmt.Sprintf("task-%s", petname.Generate(2, "-"))
		nameIsRandom = true
	}
	return &Task{
		name:           opts.Name,
		description:    opts.Description,
		kind:           opts.Kind,
		inputs:         opts.Inputs,
		outputs:        opts.Outputs,
		timeout:        opts.Timeout,
		agent:          opts.Agent,
		expectedOutput: opts.ExpectedOutput,
		outputFormat:   opts.OutputFormat,
		outputFile:     opts.OutputFile,
		outputObject:   opts.OutputObject,
		nameIsRandom:   nameIsRandom,
	}
}

func (t *Task) Name() string                    { return t.name }
func (t *Task) Description() string             { return t.description }
func (t *Task) Kind() string                    { return t.kind }
func (t *Task) Inputs() map[string]Input        { return t.inputs }
func (t *Task) Outputs() map[string]Output      { return t.outputs }
func (t *Task) Timeout() time.Duration          { return t.timeout }
func (t *Task) Agent() dive.Agent               { return t.agent }
func (t *Task) ExpectedOutput() string          { return t.expectedOutput }
func (t *Task) OutputFormat() dive.OutputFormat { return t.outputFormat }
func (t *Task) OutputFile() string              { return t.outputFile }
func (t *Task) OutputObject() interface{}       { return t.outputObject }

// Validate checks if the task is properly configured
func (t *Task) Validate() error {
	if t.name == "" {
		return fmt.Errorf("task name required")
	}
	if t.description == "" {
		return fmt.Errorf("description required for task %q", t.name)
	}
	if t.outputObject != nil && t.outputFormat != dive.OutputJSON {
		return fmt.Errorf("expected json output format for task %q", t.name)
	}
	if t.outputFile != "" {
		if !isSimpleFilename(t.outputFile) {
			return fmt.Errorf("output file name %q contains invalid characters", t.outputFile)
		}
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
	if t.expectedOutput != "" {
		lines = append(lines, fmt.Sprintf("Please respond with %s.", t.expectedOutput))
	}
	if t.outputFormat != "" {
		lines = append(lines, fmt.Sprintf("Your response must be in %s format.", t.outputFormat))
	}
	promptParts := []string{
		formatBlock(intro, "task", strings.Join(lines, "\n\n")),
	}
	var contextParts []string
	// if t.context != "" {
	// 	contextParts = append(contextParts, t.context)
	// }
	if opts.Context != "" {
		contextParts = append(contextParts, opts.Context)
	}
	if len(contextParts) > 0 {
		contextHeading := "Use this context while working on the task:"
		promptParts = append(promptParts, formatBlock(contextHeading, "context", strings.Join(contextParts, "\n\n")))
	}
	// if t.depOutput != "" {
	// 	depHeading := "Here is the output from this task's dependencies:"
	// 	promptParts = append(promptParts, formatBlock(depHeading, "dependencies", t.depOutput))
	// }
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
