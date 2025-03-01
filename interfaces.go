package dive

import (
	"context"
	"encoding/json"

	"github.com/getstingrai/dive/llm"
)

type Event struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// Team is a collection of agents that work together to complete tasks
type Team interface {
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
	HandleEvent(ctx context.Context, event *Event) error

	// Work on one or more tasks. The returned stream can be read from
	// asynchronously to receive events and task results.
	Work(ctx context.Context, tasks ...*Task) (Stream, error)

	// Start all agents belonging to the team
	Start(ctx context.Context) error

	// Stop all agents belonging to the team
	Stop(ctx context.Context) error

	// IsRunning returns true if the team is running
	IsRunning() bool
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

	// Generate a response from the agent
	Generate(ctx context.Context, message *llm.Message, opts ...GenerateOption) (*llm.Response, error)

	// Stream a response from the agent
	Stream(ctx context.Context, message *llm.Message, opts ...GenerateOption) (Stream, error)
}

// TeamAgent is an Agent that can join a team and work on tasks
type TeamAgent interface {
	Agent

	// Team the agent belongs to
	Team() Team

	// Join a team. This is only valid if the agent is not yet running and is
	// not yet a member of any team.
	Join(team Team) error

	// IsSupervisor returns true if the agent is a supervisor
	IsSupervisor() bool

	// Subordinates returns the names of the agents that the agent can supervise
	Subordinates() []string

	// Work gives the agent a task to complete and returns a stream of events
	Work(ctx context.Context, task *Task) (Stream, error)
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

// EventedAgent is an Agent that can handle events
type EventedAgent interface {
	Agent

	// AcceptedEvents returns the names of supported events
	AcceptedEvents() []string

	// HandleEvent passes an event to the event handler
	HandleEvent(ctx context.Context, event *Event) error
}

// Stream provides access to a stream of events from a Team or Agent
type Stream interface {
	// Channel returns the channel to be used to receive events
	Channel() <-chan *StreamEvent

	// Close closes the stream
	Close()
}

// StreamEvent is an event from a Stream
type StreamEvent struct {
	// Type of the event
	Type string `json:"type"`

	// TaskName is the name of the task that generated the event, if any
	TaskName string `json:"task_name,omitempty"`

	// AgentName is the name of the agent associated with the event, if any
	AgentName string `json:"agent_name,omitempty"`

	// Data contains the event payload
	Data json.RawMessage `json:"data,omitempty"`

	// Error contains an error message if the event is an error
	Error string `json:"error,omitempty"`

	// TaskResult is the result of a task, if the event is a task result
	TaskResult *TaskResult `json:"task_result,omitempty"`
}

// TODO: TaskResult should not hold a Task. Make it JSON serializable?
