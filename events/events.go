package events

import (
	"context"
	"fmt"
)

// Origin carries information about what produced the event
type Origin struct {
	AgentID         string `json:"agent_id,omitempty"`
	AgentName       string `json:"agent_name,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	TaskName        string `json:"task_name,omitempty"`
	WorkflowID      string `json:"workflow_id,omitempty"`
	WorkflowName    string `json:"workflow_name,omitempty"`
	EnvironmentID   string `json:"environment_id,omitempty"`
	EnvironmentName string `json:"environment_name,omitempty"`
}

// Event is an event that carries LLM events, task results, or errors.
type Event struct {
	// Type of the event
	Type string

	// Origin is the origin of the event
	Origin Origin

	// Payload carries additional data for the event
	Payload any

	// Error conveys an error message
	Error error
}

// TaskResult holds the output of a completed task
// type TaskResult struct {
// 	Task    interface{}
// 	Content string
// 	Object  interface{}
// 	Usage   interface{}
// }

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

// WaitForEvent waits for an event with a payload of the specified type and returns it.
// It will return an error if the context is canceled or if an error event is received.
func WaitForEvent[T any](ctx context.Context, stream Stream) (T, error) {
	var result T
	for {
		select {
		case event := <-stream.Channel():
			if event == nil {
				return result, fmt.Errorf("received nil event from stream")
			}
			if event.Error != nil {
				return result, fmt.Errorf("received error event from stream: %w", event.Error)
			}
			if payload, ok := event.Payload.(T); ok {
				return payload, nil
			}

		case <-ctx.Done():
			return result, ctx.Err()
		}
	}
}
