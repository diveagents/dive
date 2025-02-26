package dive

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/getstingrai/dive/llm"
)

type Event struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Role struct {
	Description   string
	IsSupervisor  bool
	Subordinates  []string
	AcceptsChats  bool
	AcceptsEvents []string
	AcceptsWork   []string
}

func (r Role) String() string {
	var lines []string
	result := strings.TrimSpace(r.Description)
	if result != "" {
		if !strings.HasSuffix(result, ".") {
			result += "."
		}
		lines = append(lines, result)
	}
	if r.IsSupervisor {
		lines = append(lines, "You are a supervisor. You can assign work to other agents.")
	}
	if len(lines) > 0 {
		result = strings.Join(lines, "\n\n")
	}
	return result
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

	// Event passes an event to the team
	Event(ctx context.Context, event *Event) error

	// Work on tasks
	Work(ctx context.Context, tasks ...*Task) (Stream, error)

	// Start all agents belonging to the team
	Start(ctx context.Context) error

	// Stop all agents belonging to the team
	Stop(ctx context.Context) error

	// IsRunning returns true if the team is running
	IsRunning() bool
}

// Agent is an entity that can perform tasks and interact with the world
type Agent interface {
	// Name of the agent
	Name() string

	// Role returns the agent's assigned role
	Role() Role

	// Join a team. This is only valid if the agent is not yet running and is
	// not yet a member of any team.
	Join(team Team) error

	// Team the agent belongs to. Returns nil if the agent is not on a team.
	Team() Team

	// Chat with the agent
	Chat(ctx context.Context, message *llm.Message) (*llm.Response, error)

	// ChatStream streams the conversation between the agent and the user
	ChatStream(ctx context.Context, message *llm.Message) (llm.Stream, error)

	// Event passes an event to the agent
	Event(ctx context.Context, event *Event) error

	// Work gives the agent a task to complete
	Work(ctx context.Context, task *Task) (Stream, error)

	// Start the agent
	Start(ctx context.Context) error

	// Stop the agent
	Stop(ctx context.Context) error

	// IsRunning returns true if the agent is running
	IsRunning() bool
}

// Stream is a stream of events from a Team or Agent
type Stream interface {
	// Events returns a channel of events from the stream
	Events() <-chan *StreamEvent

	// Result returns a channel that will receive task results
	Results() <-chan *TaskResult

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
}
