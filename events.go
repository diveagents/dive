package dive

import (
	"context"
	"fmt"
)

// EventOrigin carries information about what produced the event
type EventOrigin struct {
	AgentID         string `json:"agent_id,omitempty"`
	AgentName       string `json:"agent_name,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	TaskName        string `json:"task_name,omitempty"`
	WorkflowID      string `json:"workflow_id,omitempty"`
	WorkflowName    string `json:"workflow_name,omitempty"`
	EnvironmentID   string `json:"environment_id,omitempty"`
	EnvironmentName string `json:"environment_name,omitempty"`
}

// Event generated by a Dive Agent or Workflow Execution.
type Event struct {
	// Type of the event
	Type string

	// Origin describes what produced the Event
	Origin EventOrigin

	// Payload contains arbitrary data associated with the Event
	Payload any

	// Error is set if this Event corresponds to an error
	Error error
}

// Stream is an interface used to consume Events.
type Stream interface {
	// Next advances the stream to the next event. It returns false when the stream
	// is complete or if an error occurs. The caller should check Err() after Next
	// returns false to distinguish between normal completion and errors.
	Next(ctx context.Context) bool

	// Event returns the current event in the stream. It should only be called
	// after a successful call to Next.
	Event() *Event

	// Err returns any error that occurred while reading from the stream.
	// It should be checked after Next returns false.
	Err() error

	// Close closes the stream and releases any associated resources.
	Close() error

	// Publisher returns a publisher for the stream
	Publisher() Publisher
}

// Publisher is an interface used to send events.
type Publisher interface {
	// Send sends an event to the stream
	Send(ctx context.Context, event *Event) error

	// Close closes the publisher and releases any resources
	Close()
}

// WaitForEvent waits for an event with a payload of the specified type and returns it.
// It will return an error if the context is canceled or if an error event is received.
func WaitForEvent[T any](ctx context.Context, stream Stream) (T, error) {
	var result T
	for stream.Next(ctx) {
		event := stream.Event()
		if event == nil {
			return result, fmt.Errorf("received nil event from stream")
		}
		if event.Error != nil {
			return result, fmt.Errorf("received error event from stream: %w", event.Error)
		}
		if payload, ok := event.Payload.(T); ok {
			return payload, nil
		}
	}
	if err := stream.Err(); err != nil {
		return result, err
	}
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
		return result, fmt.Errorf("stream completed without finding matching event")
	}
}

// streamImpl implements the Stream interface
type streamImpl struct {
	ch     chan *Event
	curr   *Event
	err    error
	closed bool
	pub    Publisher
}

// NewStream creates a new event stream
func NewStream() Stream {
	ch := make(chan *Event, 16)
	s := &streamImpl{ch: ch}
	s.pub = newPublisher(s)
	return s
}

func (s *streamImpl) Next(ctx context.Context) bool {
	var ok bool
	var event *Event
	select {
	case <-ctx.Done():
		s.closed = true
		s.err = ctx.Err()
		return false
	case event, ok = <-s.ch:
		if !ok {
			s.closed = true
			return false
		}
	}
	s.curr = event
	return true
}

func (s *streamImpl) Event() *Event {
	return s.curr
}

func (s *streamImpl) Err() error {
	return s.err
}

func (s *streamImpl) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.ch)
	return nil
}

func (s *streamImpl) Publisher() Publisher {
	return s.pub
}

// publisherImpl is the concrete implementation of the Publisher interface
type publisherImpl struct {
	stream *streamImpl
	closed bool
}

func newPublisher(stream *streamImpl) Publisher {
	return &publisherImpl{
		stream: stream,
	}
}

func (p *publisherImpl) Send(ctx context.Context, event *Event) error {
	if p.closed {
		return fmt.Errorf("publisher is closed")
	}

	select {
	case p.stream.ch <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *publisherImpl) Close() {
	if p.closed {
		return
	}
	p.closed = true
	p.stream.Close()
}
