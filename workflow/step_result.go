package workflow

import (
	"time"

	"github.com/getstingrai/dive/llm"
)

// OutputFormat defines the format of step results
type OutputFormat string

const (
	OutputText     OutputFormat = "text"
	OutputMarkdown OutputFormat = "markdown"
	OutputJSON     OutputFormat = "json"
)

type StepStatus string

const (
	StepStatusQueued    StepStatus = "queued"
	StepStatusActive    StepStatus = "active"
	StepStatusPaused    StepStatus = "paused"
	StepStatusCompleted StepStatus = "completed"
	StepStatusBlocked   StepStatus = "blocked"
	StepStatusError     StepStatus = "error"
	StepStatusInvalid   StepStatus = "invalid"
)

// StepResult holds the output of a completed step
type StepResult struct {
	// Step is the step that was executed
	Step *Step

	// Raw content of the output
	Content string

	// Format specifies how to interpret the content
	Format OutputFormat

	// For JSON outputs, Object is the parsed JSON object
	Object interface{}

	// Reasoning is the thought process used to arrive at the answer
	Reasoning string

	// Error is the error that occurred during task execution
	Error error

	// StartedAt is the time the task was started
	StartedAt time.Time

	// FinishedAt is the time the task stopped
	FinishedAt time.Time

	// Usage is the usage of the LLM
	Usage llm.Usage
}

func NewStepResultError(step *Step, err error) *StepResult {
	return &StepResult{
		Step:  step,
		Error: err,
	}
}
