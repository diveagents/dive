package dive

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var ErrStreamClosed = errors.New("stream is closed")

// EventOrigin carries information about what produced the event.
type EventOrigin struct {
	AgentName       string `json:"agent_name,omitempty"`
	TaskName        string `json:"task_name,omitempty"`
	WorkflowName    string `json:"workflow_name,omitempty"`
	EnvironmentName string `json:"environment_name,omitempty"`
}

// Event generated by an Agent or Workflow.
type Event struct {
	// Type of the event
	Type string

	// Origin describes what produced the Event
	Origin EventOrigin

	// Error is set if this Event corresponds to an error
	Error error

	// Payload contains arbitrary data associated with the Event
	Payload any
}

// EventStream is an interface used to consume events from Agents and Workflows.
// An EventStream should be consumed by a single goroutine. These functions are
// not thread safe.
type EventStream interface {
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

	// Close the stream and release any associated resources.
	Close() error
}

// EventPublisher is an interface used to send events to an EventStream.
// These functions are safe to call concurrently.
type EventPublisher interface {
	// Send sends an event to the stream
	Send(ctx context.Context, event *Event) error

	// Close closes the publisher and the referenced stream. This signals to the
	// consumer that no more events will be sent.
	Close()
}

// WaitForEvent waits for an event with a payload of the specified type and returns it.
// It will return an error if the context is canceled or if an error event is received.
func WaitForEvent[T any](ctx context.Context, stream EventStream) (T, error) {
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

type eventStream struct {
	ch        chan *Event
	curr      *Event
	err       error
	closed    bool
	pub       EventPublisher
	closeOnce sync.Once
}

// NewEventStream returns a new event stream and a publisher for the stream.
func NewEventStream() (EventStream, EventPublisher) {
	s := &eventStream{ch: make(chan *Event, 16)}
	p := &eventPublisher{
		stream: s,
		done:   make(chan struct{}),
	}
	s.pub = p
	return s, p
}

func (s *eventStream) Next(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		s.closed = true
		s.err = ctx.Err()
		return false
	case event, ok := <-s.ch:
		if !ok {
			s.closed = true
			return false
		}
		s.curr = event
		return true
	}
}

func (s *eventStream) Event() *Event {
	return s.curr
}

func (s *eventStream) Err() error {
	return s.err
}

func (s *eventStream) Close() error {
	s.closeOnce.Do(func() {
		s.closed = true
		s.pub.Close()
	})
	return nil
}

type eventPublisher struct {
	stream *eventStream
	done   chan struct{}
}

func (p *eventPublisher) Send(ctx context.Context, event *Event) error {
	select {
	case <-p.done:
		return ErrStreamClosed
	default:
		select {
		case <-p.done:
			return ErrStreamClosed
		case <-ctx.Done():
			return ctx.Err()
		case p.stream.ch <- event:
			return nil
		}
	}
}

func (p *eventPublisher) Close() {
	select {
	case <-p.done:
		return
	default:
		close(p.done)
		close(p.stream.ch)
	}
}
