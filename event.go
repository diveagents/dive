package dive

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/diveagents/dive/llm"
)

var ErrStreamClosed = errors.New("stream is closed")

// EventOrigin carries information about what produced the event.
type EventOrigin struct {
	AgentName       string `json:"agent_name,omitempty"`
	TaskName        string `json:"task_name,omitempty"`
	WorkflowName    string `json:"workflow_name,omitempty"`
	EnvironmentName string `json:"environment_name,omitempty"`
}

// EventType is the type of event emitted by an Agent or Workflow.
type EventType string

const (
	// EventTypeGenerationStarted   EventType = "generation.started"
	// EventTypeGenerationProgress  EventType = "generation.progress"
	// EventTypeGenerationCompleted EventType = "generation.completed"
	// EventTypeGenerationError     EventType = "generation.error"
	// EventTypeLLMEvent            EventType = "llm.event"
	// EventTypeToolCalled          EventType = "tool.called"
	// EventTypeToolOutput          EventType = "tool.output"
	// EventTypeToolError           EventType = "tool.error"
	// EventTypeTaskActivated       EventType = "task.activated"
	// EventTypeTaskProgress        EventType = "task.progress"
	// EventTypeTaskPaused          EventType = "task.paused"
	// EventTypeTaskCompleted       EventType = "task.completed"
	// EventTypeTaskError           EventType = "task.error"
	// EventTypeError               EventType = "error"

	EventTypeResponseCreated    EventType = "response.created"
	EventTypeResponseInProgress EventType = "response.in_progress"
	EventTypeResponseCompleted  EventType = "response.completed"
	EventTypeResponseFailed     EventType = "response.failed"
	EventTypeResponseToolCall   EventType = "response.tool_call"
	EventTypeResponseToolResult EventType = "response.tool_result"
	EventTypeResponseToolError  EventType = "response.tool_error"
	EventTypeLLMEvent           EventType = "llm.event"
	EventTypeError              EventType = "error"
)

func (t EventType) String() string {
	return string(t)
}

// ResponseEvent carries
type ResponseEvent struct {
	// Type of the event
	Type EventType `json:"type"`

	// Error is set if this Event corresponds to an error
	Error error `json:"error,omitempty"`

	// Item contains the item for item events
	Item *ResponseItem `json:"item,omitempty"`

	// Response contains the complete response for response completed events
	Response *Response `json:"response,omitempty"`
}

// // ResponseStream is an interface used to consume events from Agents and Workflows.
// // An ResponseStream should be consumed by a single goroutine. These functions are
// // not thread safe.
// type ResponseStream interface {
// 	// Next advances the stream to the next event. It returns false when the stream
// 	// is complete or if an error occurs. The caller should check Err() after Next
// 	// returns false to distinguish between normal completion and errors.
// 	Next(ctx context.Context) bool

// 	// Event returns the current event in the stream. It should only be called
// 	// after a successful call to Next.
// 	Event() *Event

// 	// Err returns any error that occurred while reading from the stream.
// 	// It should be checked after Next returns false.
// 	Err() error

// 	// Close the stream and release any associated resources.
// 	Close() error
// }

// EventPublisher is an interface used to send events to an EventStream.
// These functions are safe to call concurrently.
type EventPublisher interface {
	// Send sends an event to the stream
	Send(ctx context.Context, event *ResponseEvent) error

	// Close closes the publisher and the referenced stream. This signals to the
	// consumer that no more events will be sent.
	Close()
}

// ReadEventPayloads waits for and returns all events with a payload of the specified type.
// It will return an error if the context is canceled or if an error event is received.
func ReadEventPayloads[T any](ctx context.Context, stream ResponseStream) ([]T, error) {
	var results []T
	for stream.Next(ctx) {
		event := stream.Event()
		if event == nil {
			return results, fmt.Errorf("received nil event from stream")
		}
		if event.Error != nil {
			return results, fmt.Errorf("received error event from stream: %w", event.Error)
		}
		// This needs to be implemented by the caller based on the ResponseEvent structure
		// as ResponseEvent no longer has a generic Payload field
	}
	if err := stream.Err(); err != nil {
		return results, err
	}
	return results, nil
}

// ReadMessages returns all messages generated in an interaction. This will include
// both assistant messages and tool result messages.
func ReadMessages(ctx context.Context, stream ResponseStream) ([]*llm.Message, error) {
	var messages []*llm.Message
	for stream.Next(ctx) {
		event := stream.Event()
		if event == nil {
			return nil, fmt.Errorf("received nil event from stream")
		}
		if event.Error != nil {
			return nil, fmt.Errorf("received error event from stream: %w", event.Error)
		}
		if event.Type == EventTypeResponseCompleted && event.Response != nil {
			for _, item := range event.Response.Items {
				if item.Type == ResponseItemTypeMessage && item.Message != nil {
					messages = append(messages, item.Message)
				}
			}
			return messages, nil
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("stream ended without a response completed event")
}

type responseEventStream struct {
	ch        chan *ResponseEvent
	curr      *ResponseEvent
	err       error
	closed    bool
	pub       EventPublisher
	closeOnce sync.Once
}

// NewEventStream returns a new event stream and a publisher for the stream.
func NewEventStream() (ResponseStream, EventPublisher) {
	s := &responseEventStream{ch: make(chan *ResponseEvent, 16)}
	p := &eventPublisher{
		stream: s,
		done:   make(chan struct{}),
	}
	s.pub = p
	return s, p
}

func (s *responseEventStream) Next(ctx context.Context) bool {
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

func (s *responseEventStream) Event() *ResponseEvent {
	return s.curr
}

func (s *responseEventStream) Err() error {
	return s.err
}

func (s *responseEventStream) Close() error {
	s.closeOnce.Do(func() {
		s.closed = true
		s.pub.Close()
	})
	return nil
}

type eventPublisher struct {
	stream *responseEventStream
	done   chan struct{}
}

func (p *eventPublisher) Send(ctx context.Context, event *ResponseEvent) error {
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

// Generation contains information about a Dive LLM interaction. This may have
// involved one or more underlying LLM calls, since follow up calls may be made
// to pass tool results back to the LLM.
// type Generation struct {
// 	ID              string            `json:"id,omitempty"`
// 	StartedAt       time.Time         `json:"started_at,omitempty"`
// 	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
// 	Config          *llm.Config       `json:"config,omitempty"`
// 	InputMessages   []*llm.Message    `json:"input_messages,omitempty"`
// 	OutputMessages  []*llm.Message    `json:"output_messages,omitempty"`
// 	ToolResults     []*llm.ToolResult `json:"tool_results,omitempty"`
// 	ActiveToolCalls int               `json:"active_tool_calls,omitempty"`
// 	TotalUsage      llm.Usage         `json:"total_usage,omitempty"`
// 	Error           error             `json:"error,omitempty"`
// 	IsDone          bool              `json:"is_done,omitempty"`
// 	Model           string            `json:"model,omitempty"`
// }

// func (g *Generation) AccumulateUsage(usage llm.Usage) {
// 	g.TotalUsage.InputTokens += usage.InputTokens
// 	g.TotalUsage.OutputTokens += usage.OutputTokens
// 	g.TotalUsage.CacheCreationInputTokens += usage.CacheCreationInputTokens
// 	g.TotalUsage.CacheReadInputTokens += usage.CacheReadInputTokens
// }

// func (g *Generation) LastMessage() (*llm.Message, bool) {
// 	mLen := len(g.OutputMessages)
// 	if mLen == 0 {
// 		return nil, false
// 	}
// 	return g.OutputMessages[mLen-1], true
// }
