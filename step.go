package dive

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
)

// StepOptions are used to create a Step
type StepOptions struct {
	Name           string
	Description    string
	ExpectedOutput string
	OutputFormat   OutputFormat
	OutputFile     string
	Dependencies   []string
	Timeout        time.Duration
	Context        string
	OutputObject   interface{}
	AssignedAgent  Agent
	DocumentRefs   []DocumentRef
}

// Step represents one step in a workflow
type Step struct {
	name           string
	description    string
	expectedOutput string
	outputFormat   OutputFormat
	outputObject   interface{}
	assignedAgent  Agent
	dependencies   []string
	documentRefs   []DocumentRef
	outputFile     string
	result         *StepResult
	timeout        time.Duration
	context        string
	depOutput      string
	nameIsRandom   bool
}

// NewStep creates a new Step from a StepOptions
func NewStep(opts StepOptions) *Step {
	var nameIsRandom bool
	if opts.Name == "" {
		opts.Name = fmt.Sprintf("task-%s", petname.Generate(2, "-"))
		nameIsRandom = true
	}
	return &Step{
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

func (s *Step) Name() string                { return s.name }
func (s *Step) Description() string         { return s.description }
func (s *Step) ExpectedOutput() string      { return s.expectedOutput }
func (s *Step) OutputFormat() OutputFormat  { return s.outputFormat }
func (s *Step) OutputObject() interface{}   { return s.outputObject }
func (s *Step) AssignedAgent() Agent        { return s.assignedAgent }
func (s *Step) Dependencies() []string      { return s.dependencies }
func (s *Step) DocumentRefs() []DocumentRef { return s.documentRefs }
func (s *Step) OutputFile() string          { return s.outputFile }
func (s *Step) Result() *StepResult         { return s.result }
func (s *Step) Timeout() time.Duration      { return s.timeout }
func (s *Step) Context() string             { return s.context }
func (s *Step) DependenciesOutput() string  { return s.depOutput }

func (s *Step) SetContext(ctx string)               { s.context = ctx }
func (s *Step) SetDependenciesOutput(output string) { s.depOutput = output }
func (s *Step) SetResult(result *StepResult)        { s.result = result }
func (s *Step) SetAssignedAgent(agent Agent)        { s.assignedAgent = agent }

// Validate checks if the step is properly configured
func (s *Step) Validate() error {
	if s.name == "" {
		return fmt.Errorf("step name required")
	}
	if s.description == "" {
		return fmt.Errorf("description required for step %q", s.name)
	}
	if s.outputObject != nil && s.outputFormat != OutputJSON {
		return fmt.Errorf("expected json output format for step %q", s.name)
	}
	if s.outputFile != "" {
		if !isSimpleFilename(s.outputFile) {
			return fmt.Errorf("output file name %q contains invalid characters", s.outputFile)
		}
	}
	for _, depID := range s.dependencies {
		if depID == s.name {
			return fmt.Errorf("step %q cannot depend on itself", s.name)
		}
	}
	return nil
}

type StepPromptOptions struct {
	Context string
}

// Prompt returns the LLM prompt for the task
func (s *Step) Prompt(opts ...*StepPromptOptions) string {
	var options *StepPromptOptions
	if len(opts) > 0 {
		options = opts[0]
	}
	var intro string
	if s.name != "" && !s.nameIsRandom {
		intro = fmt.Sprintf("Let's work on a new task named %q.", s.name)
	} else {
		intro = "Let's work on a new task."
	}
	lines := []string{}
	if s.description != "" {
		lines = append(lines, s.description)
	}
	if s.expectedOutput != "" {
		lines = append(lines, fmt.Sprintf("Please respond with %s.", s.expectedOutput))
	}
	if s.outputFormat != "" {
		lines = append(lines, fmt.Sprintf("Your response must be in %s format.", s.outputFormat))
	}
	promptParts := []string{
		formatBlock(intro, "task", strings.Join(lines, "\n\n")),
	}
	var contextParts []string
	if s.context != "" {
		contextParts = append(contextParts, s.context)
	}
	if options != nil && options.Context != "" {
		contextParts = append(contextParts, options.Context)
	}
	if len(contextParts) > 0 {
		contextHeading := "Use this context while working on the task:"
		promptParts = append(promptParts, formatBlock(contextHeading, "context", strings.Join(contextParts, "\n\n")))
	}
	if s.depOutput != "" {
		depHeading := "Here is the output from this task's dependencies:"
		promptParts = append(promptParts, formatBlock(depHeading, "dependencies", s.depOutput))
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
