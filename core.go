package dive

import (
	"context"

	"github.com/getstingrai/dive/events"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
)

var (
	DefaultLogger = slogger.NewDevNullLogger()
)

// OutputFormat defines the format of task results
type OutputFormat string

const (
	OutputText     OutputFormat = "text"
	OutputMarkdown OutputFormat = "markdown"
	OutputJSON     OutputFormat = "json"
)

type TaskStatus string

const (
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusActive    TaskStatus = "active"
	TaskStatusPaused    TaskStatus = "paused"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusBlocked   TaskStatus = "blocked"
	TaskStatusError     TaskStatus = "error"
	TaskStatusInvalid   TaskStatus = "invalid"
)

type TaskPromptOptions struct {
	Context string
}

// Input defines an expected input parameter
type Input struct {
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Output defines an expected output parameter
type Output struct {
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Format      string      `json:"format,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Task represents a unit of work that can be executed
type Task interface {
	// Name returns the name of the task
	Name() string

	// Description returns the description of the task
	Description() string

	// Agent returns the agent assigned to this task, if any
	Agent() Agent

	// Validate checks if the task is properly configured
	Validate() error

	// Inputs returns the inputs for the task
	Inputs() map[string]Input

	// Outputs returns the outputs for the task
	Outputs() map[string]Output

	// Prompt returns the prompt for the task
	Prompt(opts TaskPromptOptions) string
}

// TaskResult holds the output of a completed task
type TaskResult struct {
	// Task is the task that was executed
	Task Task

	// Content contains the raw output
	Content string

	// Format specifies how to interpret the content
	Format OutputFormat

	// Object holds parsed JSON output if applicable
	Object interface{}

	// Error is set if task execution failed
	Error error

	// Usage tracks LLM token usage
	Usage llm.Usage
}

// Agent represents an AI agent that can perform tasks
type Agent interface {
	// Name returns the agent's name
	Name() string

	// Description returns the agent's description
	Description() string

	// Instructions returns the agent's base instructions
	Instructions() string

	// IsSupervisor returns true if the agent is a supervisor
	IsSupervisor() bool

	// Subordinates returns names of agents this one can supervise
	Subordinates() []string

	// Work gives the agent a task to complete
	Work(ctx context.Context, task Task) (events.Stream, error)

	// Generate gives the agent a message to respond to
	Generate(ctx context.Context, message *llm.Message, opts ...GenerateOption) (*llm.Response, error)

	// Stream gives the agent a message to respond to and returns a stream of events
	Stream(ctx context.Context, message *llm.Message, opts ...GenerateOption) (events.Stream, error)

	// SetEnvironment sets the environment for the agent
	SetEnvironment(env Environment)
}

// RunnableAgent is an Agent that can be started and stopped
type RunnableAgent interface {
	Agent

	// Start the agent
	Start(ctx context.Context) error

	// Stop the agent
	Stop(ctx context.Context) error

	// IsRunning returns true if the agent is running
	IsRunning() bool
}

// EventHandlerAgent is an Agent that can handle events
type EventHandlerAgent interface {
	Agent

	// AcceptedEvents returns the names of supported events
	AcceptedEvents() []string

	// HandleEvent passes an event to the event handler
	HandleEvent(ctx context.Context, event *events.Event) error
}

type Environment interface {
	Name() string
	Agents() []Agent
	RegisterAgent(agent Agent) error
	GetAgent(name string) (Agent, error)
}
