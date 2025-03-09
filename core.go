package dive

import (
	"context"

	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/stream"
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

type Workflow interface {
	// Name of the team
	Name() string

	// Description of the team
	Description() string

	// Overview of the team
	Overview() (string, error)

	// Agents belonging to the team
	Agents() []Agent

	// GetAgent returns an agent by name
	GetAgent(name string) (Agent, bool)

	// HandleEvent passes an event to the team
	HandleEvent(ctx context.Context, event *stream.Event) error

	// Work on one or more tasks. The returned stream can be read from
	// asynchronously to receive events and task results.
	Work(ctx context.Context, tasks ...Task) (*stream.Stream, error)

	// Start all agents belonging to the team
	Start(ctx context.Context) error

	// Stop all agents belonging to the team
	Stop(ctx context.Context) error

	// IsRunning returns true if the team is running
	IsRunning() bool

	// DocumentStore returns the document store for the team
	DocumentStore() document.Repository
}

type generateOptions struct {
	ThreadID string
	UserID   string
}

type GenerateOption func(*generateOptions)

func WithThreadID(threadID string) GenerateOption {
	return func(opts *generateOptions) {
		opts.ThreadID = threadID
	}
}

func WithUserID(userID string) GenerateOption {
	return func(opts *generateOptions) {
		opts.UserID = userID
	}
}

// Agent is an entity that can perform tasks and interact with the world
type Agent interface {
	// Name of the agent
	Name() string

	// Description of the agent
	Description() string

	// Instructions for the agent
	Instructions() string

	// Fingerprint of the agent captures the current state of the agent
	Fingerprint() string

	// Generate a response from the agent
	Generate(ctx context.Context, message *llm.Message, opts ...GenerateOption) (*llm.Response, error)

	// Stream a response from the agent
	Stream(ctx context.Context, message *llm.Message, opts ...GenerateOption) (*stream.Stream, error)
}

// TeamAgent is an Agent that can join a team and work on tasks
type TeamAgent interface {
	Agent

	// Team the agent belongs to
	Team() Workflow

	// Join a team. This is only valid if the agent is not yet running and is
	// not yet a member of any team.
	Join(team Workflow) error

	// IsSupervisor returns true if the agent is a supervisor
	IsSupervisor() bool

	// Subordinates returns the names of the agents that the agent can supervise
	Subordinates() []string

	// Work gives the agent a task to complete and returns a stream of events
	Work(ctx context.Context, task Task) (*stream.Stream, error)
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

// Task represents a unit of work that can be executed by an agent
type Task interface {
	// Name returns the name of the task
	Name() string

	// Description returns the description of the task
	Description() string

	// ExpectedOutput returns what output is expected from this task
	ExpectedOutput() string

	// Dependencies returns the names of tasks that must be completed before this one
	Dependencies() []string

	// AssignedAgent returns the agent assigned to this task, if any
	AssignedAgent() Agent

	// Validate checks if the task is properly configured
	Validate() error
}
