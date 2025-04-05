package dive

import (
	"context"
	"strings"
	"time"

	"github.com/diveagents/dive/llm"
)

type ResponseItemType string

const (
	ResponseItemTypeMessage    ResponseItemType = "message"
	ResponseItemTypeToolCall   ResponseItemType = "tool_call"
	ResponseItemTypeToolResult ResponseItemType = "tool_result"
)

// ResponseItem contains either a message, tool call, or tool result. Multiple
// items may be generated in response to a single prompt.
type ResponseItem struct {
	// Type of the response item
	Type ResponseItemType `json:"type,omitempty"`

	// Event is set if the response item is an event
	Event *llm.Event `json:"event,omitempty"`

	// Message is set if the response item is a message
	Message *llm.Message `json:"message,omitempty"`

	// ToolCall is set if the response item is a tool call
	ToolCall *llm.ToolCall `json:"tool_call,omitempty"`

	// ToolResult is set if the response item is a tool result
	ToolResult *llm.ToolResult `json:"tool_result,omitempty"`

	// Usage contains token usage information, if applicable
	Usage *llm.Usage `json:"usage,omitempty"`
}

// Response represents the output from an Agent's response generation.
type Response struct {
	// ID is a unique identifier for this response
	ID string `json:"id,omitempty"`

	// Model represents the model that generated the response
	Model string `json:"model,omitempty"`

	// Items contains the individual response items including
	// messages, tool calls, and tool results.
	Items []*ResponseItem `json:"items,omitempty"`

	// Usage contains token usage information
	Usage *llm.Usage `json:"usage,omitempty"`

	// CreatedAt is the timestamp when this response was created
	CreatedAt time.Time `json:"created_at,omitempty"`

	// FinishedAt is the timestamp when this response was completed
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// OutputText returns all text content from all messages in the response. If
// there were multiple messages in the response, they are concatenated together
// with a double newline.
func (r *Response) OutputText() string {
	var messages []string
	for _, item := range r.Items {
		if item.Type == ResponseItemTypeMessage {
			for _, content := range item.Message.Content {
				if content.Type == llm.ContentTypeText {
					messages = append(messages, content.Text)
				}
			}
		}
	}
	return strings.Join(messages, "\n\n")
}

// ToolResults returns all tool results from the response.
func (r *Response) ToolResults() []*llm.ToolResult {
	var results []*llm.ToolResult
	for _, item := range r.Items {
		if item.Type == ResponseItemTypeToolResult {
			results = append(results, item.ToolResult)
		}
	}
	return results
}

// ResponseStream is a generic interface for streaming responses
type ResponseStream interface {
	// Next advances the stream to the next item
	Next(ctx context.Context) bool

	// Event returns the current event in the stream
	Event() *ResponseEvent

	// Err returns any error encountered while streaming
	Err() error

	// Close releases any resources associated with the stream
	Close() error
}
