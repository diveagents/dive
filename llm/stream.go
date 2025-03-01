package llm

import "context"

// StreamEventType represents the type of streaming event
type StreamEventType string

func (e StreamEventType) String() string {
	return string(e)
}

const (
	EventPing              StreamEventType = "ping"
	EventMessageStart      StreamEventType = "message_start"
	EventMessageDelta      StreamEventType = "message_delta"
	EventMessageStop       StreamEventType = "message_stop"
	EventContentBlockStart StreamEventType = "content_block_start"
	EventContentBlockDelta StreamEventType = "content_block_delta"
	EventContentBlockStop  StreamEventType = "content_block_stop"
)

// StreamEvent represents a single streaming event from the LLM. A successfully
// run stream will end with a final message containing the complete Response.
type StreamEvent struct {
	Type         StreamEventType `json:"type"`
	Index        int             `json:"index"`
	Message      *Message        `json:"message,omitempty"`
	ContentBlock *ContentBlock   `json:"content_block,omitempty"`
	Delta        *Delta          `json:"delta,omitempty"`
	Usage        *Usage          `json:"usage,omitempty"`
	Response     *Response       `json:"response,omitempty"`
}

// Stream represents a stream of LLM generation events
type Stream interface {
	// Next returns the next event in the stream. Returns nil when the stream is
	// complete or if an error occurs. Errors can be retrieved via the Err method.
	Next(ctx context.Context) (*StreamEvent, bool)

	// Err returns any error that occurred while reading from the stream
	Err() error

	// Close closes the stream and releases any associated resources
	Close() error
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Delta struct {
	Type         string  `json:"type,omitempty"`
	Text         string  `json:"text,omitempty"`
	Index        int     `json:"index,omitempty"`
	StopReason   string  `json:"stop_reason,omitempty"`
	StopSequence *string `json:"stop_sequence,omitempty"`
	PartialJSON  string  `json:"partial_json,omitempty"`
}
