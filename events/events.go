package events

import "context"

// Event is an event that carries LLM events, task results, or errors.
type Event struct {
	// Type of the event
	Type string

	// AgentName is the name of the agent associated with the event
	AgentName string

	// TaskName is the name of the task that generated the event
	TaskName string

	// LLMEvent is the event from the LLM (may be nil)
	LLMEvent interface{}

	// TaskResult is the result of a task (may be nil)
	// TaskResult *TaskResult

	// Response is the final response from the agent (may be nil)
	Response interface{}

	// Error conveys an error message
	Error string

	// Payload carries additional data for the event
	Payload any
}

// TaskResult holds the output of a completed task
type TaskResult struct {
	Task    interface{}
	Content string
	Object  interface{}
	Usage   interface{}
}

// Stream is an interface for receiving events
type Stream interface {
	// Channel returns a channel that can be used to receive events
	Channel() <-chan *Event

	// Close closes the stream and releases any resources
	Close()

	// Publisher returns a publisher for the stream
	Publisher() Publisher
}

// Publisher is an interface for sending events
type Publisher interface {
	// Send sends an event to the stream
	Send(ctx context.Context, event *Event) error

	// Close closes the publisher and releases any resources
	Close()
}

// Handler is an interface for handling events
type Handler interface {
	// HandleEvent handles an event
	HandleEvent(event *Event) error
}
