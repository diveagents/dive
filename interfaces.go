package dive

import (
	"context"

	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/slogger"
)

var (
	DefaultLogger = slogger.NewDevNullLogger()
)

// OutputFormat defines the desired output format for a Task
type OutputFormat string

const (
	OutputText     OutputFormat = "text"
	OutputMarkdown OutputFormat = "markdown"
	OutputJSON     OutputFormat = "json"
)

// TaskStatus indicates a Task's execution status
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

type TaskPromptOptions struct {
	Context string
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

	// Name of the Agent
	Name() string

	// Description of the Agent's role and responsibilities
	Description() string

	// Instructions that guide the Agent's actions
	Instructions() string

	// IsSupervisor indicates whether the Agent can assign work to other Agents
	IsSupervisor() bool

	// Subordinates returns the names of the Agents that this Agent can supervise
	Subordinates() []string

	// SetEnvironment sets the runtime Environment to which this Agent belongs
	SetEnvironment(env Environment)

	// Generate gives the agent a message to respond to
	Generate(ctx context.Context, message *llm.Message, opts ...GenerateOption) (*llm.Response, error)

	// Stream gives the agent a message to respond to and returns a stream of events
	Stream(ctx context.Context, message *llm.Message, opts ...GenerateOption) (Stream, error)

	// Work gives the agent a task to complete
	Work(ctx context.Context, task Task) (Stream, error)
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
	HandleEvent(ctx context.Context, event *Event) error
}

// Environment is a container for running Agents and Workflow Executions.
// Interactivity between Agents is scoped to a single Environment.
type Environment interface {

	// Name of the Environment
	Name() string

	// Agents returns the list of all Agents belonging to this Environment
	Agents() []Agent

	// RegisterAgent adds an Agent to this Environment
	RegisterAgent(agent Agent) error

	// GetAgent returns the Agent with the given name, if found
	GetAgent(name string) (Agent, error)
}

// GenerateOptions contains configuration for LLM generations.
type GenerateOptions struct {
	ThreadID string
	UserID   string
}

// GenerateOption is a type signature for defining new LLM generation options.
type GenerateOption func(*GenerateOptions)

// Apply invokes any supplied options. Used internally in Dive.
func (o *GenerateOptions) Apply(opts []GenerateOption) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithThreadID associates the given conversation thread ID with a generation.
// This appends the new messages to any previous messages belonging to this thread.
func WithThreadID(threadID string) GenerateOption {
	return func(opts *GenerateOptions) {
		opts.ThreadID = threadID
	}
}

// WithUserID associates the given user ID with a generation, indicating what
// person is the speaker in the conversation.
func WithUserID(userID string) GenerateOption {
	return func(opts *GenerateOptions) {
		opts.UserID = userID
	}
}
