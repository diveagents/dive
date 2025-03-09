package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/graph"
)

// TaskOptions are used to create a Task
type TaskOptions struct {
	Name           string
	Description    string
	ExpectedOutput string
	OutputFormat   dive.OutputFormat
	OutputFile     string
	Dependencies   []string
	Timeout        time.Duration
	Context        string
	OutputObject   interface{}
	AssignedAgent  dive.Agent
	DocumentRefs   []document.DocumentRef
}

// Task represents one task in a workflow
type Task struct {
	name           string
	description    string
	expectedOutput string
	outputFormat   dive.OutputFormat
	outputObject   interface{}
	assignedAgent  dive.Agent
	dependencies   []string
	documentRefs   []document.DocumentRef
	outputFile     string
	result         *TaskResult
	timeout        time.Duration
	context        string
	depOutput      string
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
		expectedOutput: opts.ExpectedOutput,
		outputFormat:   opts.OutputFormat,
		outputObject:   opts.OutputObject,
		outputFile:     opts.OutputFile,
		assignedAgent:  opts.AssignedAgent,
		dependencies:   opts.Dependencies,
		documentRefs:   opts.DocumentRefs,
		timeout:        opts.Timeout,
		context:        opts.Context,
		nameIsRandom:   nameIsRandom,
	}
}

func (t *Task) Name() string                         { return t.name }
func (t *Task) Description() string                  { return t.description }
func (t *Task) ExpectedOutput() string               { return t.expectedOutput }
func (t *Task) OutputFormat() dive.OutputFormat      { return t.outputFormat }
func (t *Task) OutputObject() interface{}            { return t.outputObject }
func (t *Task) AssignedAgent() dive.Agent            { return t.assignedAgent }
func (t *Task) Dependencies() []string               { return t.dependencies }
func (t *Task) DocumentRefs() []document.DocumentRef { return t.documentRefs }
func (t *Task) OutputFile() string                   { return t.outputFile }
func (t *Task) Result() *TaskResult                  { return t.result }
func (t *Task) Timeout() time.Duration               { return t.timeout }
func (t *Task) Context() string                      { return t.context }
func (t *Task) DependenciesOutput() string           { return t.depOutput }

func (t *Task) SetContext(ctx string)               { t.context = ctx }
func (t *Task) SetDependenciesOutput(output string) { t.depOutput = output }
func (t *Task) SetResult(result *TaskResult)        { t.result = result }
func (t *Task) SetAssignedAgent(agent dive.Agent)   { t.assignedAgent = agent }

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
	for _, depID := range t.dependencies {
		if depID == t.name {
			return fmt.Errorf("task %q cannot depend on itself", t.name)
		}
	}
	return nil
}

type TaskPromptOptions struct {
	Context string
}

// Prompt returns the LLM prompt for the task
func (t *Task) Prompt(opts ...*TaskPromptOptions) string {
	var options *TaskPromptOptions
	if len(opts) > 0 {
		options = opts[0]
	}
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
	if t.context != "" {
		contextParts = append(contextParts, t.context)
	}
	if options != nil && options.Context != "" {
		contextParts = append(contextParts, options.Context)
	}
	if len(contextParts) > 0 {
		contextHeading := "Use this context while working on the task:"
		promptParts = append(promptParts, formatBlock(contextHeading, "context", strings.Join(contextParts, "\n\n")))
	}
	if t.depOutput != "" {
		depHeading := "Here is the output from this task's dependencies:"
		promptParts = append(promptParts, formatBlock(depHeading, "dependencies", t.depOutput))
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

// OrderTasks sorts tasks into execution order using their dependencies.
func OrderTasks(tasks []dive.Task) ([]string, error) {
	nodes := make([]graph.Node, len(tasks))
	for i, task := range tasks {
		nodes[i] = task
	}
	order, err := graph.New(nodes).TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("invalid task dependencies: %w", err)
	}
	return order, nil
}
