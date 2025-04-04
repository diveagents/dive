package dive

import (
	"context"
	"fmt"
	"time"

	"github.com/diveagents/dive/llm"
	"github.com/gofrs/uuid/v5"
)

// OutputFormat defines the desired output format for a Task
type OutputFormat string

const (
	OutputFormatText     OutputFormat = "text"
	OutputFormatMarkdown OutputFormat = "markdown"
	OutputFormatJSON     OutputFormat = "json"
)

// TaskStatus indicates a Task's execution status
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

// Input defines an expected input parameter
type Input struct {
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// Output defines an expected output parameter
type Output struct {
	Name        string      `json:"name"`
	Type        string      `json:"type,omitempty"`
	Description string      `json:"description,omitempty"`
	Format      string      `json:"format,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Document    string      `json:"document,omitempty"`
}

// PromptContext is a named block of information carried by a Prompt
type PromptContext struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Text        string `json:"text,omitempty"`
}

// Prompt is a structured representation of an LLM prompt
type Prompt struct {
	Name         string           `json:"name"`
	Text         string           `json:"text,omitempty"`
	Context      []*PromptContext `json:"context,omitempty"`
	Output       string           `json:"output,omitempty"`
	OutputFormat OutputFormat     `json:"output_format,omitempty"`
}

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

// Agent represents an intelligent agent that can work on tasks and respond to
// chat messages.
type Agent interface {

	// Name of the Agent
	Name() string

	// Goal of the Agent
	Goal() string

	// IsSupervisor indicates whether the Agent can assign work to other Agents
	IsSupervisor() bool

	// SetEnvironment sets the runtime Environment to which this Agent belongs
	SetEnvironment(env Environment) error

	// CreateResponse creates a new Response from the Agent
	CreateResponse(ctx context.Context, opts ...ChatOption) (*Response, error)

	// StreamResponse streams a new Response from the Agent
	StreamResponse(ctx context.Context, opts ...ChatOption) (ResponseStream, error)
}

// Environment is a container for running Agents and Workflows. Interactivity
// between Agents is generally scoped to a single Environment.
type Environment interface {

	// Name of the Environment
	Name() string

	// Agents returns the list of all Agents belonging to this Environment
	Agents() []Agent

	// RegisterAgent adds an Agent to this Environment
	RegisterAgent(agent Agent) error

	// GetAgent returns the Agent with the given name, if found
	GetAgent(name string) (Agent, error)

	// DocumentRepository returns the DocumentRepository for this Environment
	DocumentRepository() DocumentRepository

	// ThreadRepository returns the ThreadRepository for this Environment
	ThreadRepository() ThreadRepository
}

// ChatOptions contains configuration for LLM generations.
type ChatOptions struct {
	ThreadID      string
	UserID        string
	Messages      []*llm.Message
	Input         string
	EventCallback EventCallback
}

// EventCallback is a function that processes streaming events during response generation.
type EventCallback func(ctx context.Context, event *ResponseEvent) error

// ChatOption is a type signature for defining new LLM generation options.
type ChatOption func(*ChatOptions)

// Apply invokes any supplied options. Used internally in Dive.
func (o *ChatOptions) Apply(opts []ChatOption) {
	for _, opt := range opts {
		opt(o)
	}
}

// WithThreadID associates the given conversation thread ID with a generation.
// This appends the new messages to any previous messages belonging to this thread.
func WithThreadID(threadID string) ChatOption {
	return func(opts *ChatOptions) {
		opts.ThreadID = threadID
	}
}

// WithUserID associates the given user ID with a generation, indicating what
// person is the speaker in the conversation.
func WithUserID(userID string) ChatOption {
	return func(opts *ChatOptions) {
		opts.UserID = userID
	}
}

// WithMessages specifies the messages to be used in the generation.
func WithMessages(messages []*llm.Message) ChatOption {
	return func(opts *ChatOptions) {
		opts.Messages = messages
	}
}

// WithInput specifies a simple text input string to be used in the generation.
// This is a convenience wrapper that creates a single user message.
func WithInput(input string) ChatOption {
	return func(opts *ChatOptions) {
		opts.Input = input
	}
}

// WithEventCallback specifies a callback function that will be invoked for each
// event generated during response creation.
func WithEventCallback(callback EventCallback) ChatOption {
	return func(opts *ChatOptions) {
		opts.EventCallback = callback
	}
}

// Task represents a unit of work that can be executed by an Agent.
type Task interface {
	// Name returns the name of the task
	Name() string

	// Timeout returns the maximum duration allowed for task execution
	Timeout() time.Duration

	// Prompt returns the LLM prompt for the task
	Prompt() (*Prompt, error)
}

// TaskResult holds the output of a completed task.
type TaskResult struct {
	// Task is the task that was executed
	Task Task

	// Content contains the raw output
	Content string

	// Format specifies how to interpret the content
	Format OutputFormat

	// Object holds parsed JSON output if applicable
	Object interface{}

	// Error is set if task execution failed
	Error error

	// Usage tracks LLM token usage
	Usage llm.Usage
}

// ListDocumentInput specifies search criteria for documents in a document store
type ListDocumentInput struct {
	PathPrefix string
	Recursive  bool
}

// ListDocumentOutput is the output for listing documents
type ListDocumentOutput struct {
	Items []Document
}

// DocumentRepository provides read/write access to a set of documents. This could be
// backed by a local file system, a remote API, or a database.
type DocumentRepository interface {

	// GetDocument returns a document by name
	GetDocument(ctx context.Context, name string) (Document, error)

	// ListDocuments lists documents
	ListDocuments(ctx context.Context, input *ListDocumentInput) (*ListDocumentOutput, error)

	// PutDocument puts a document
	PutDocument(ctx context.Context, doc Document) error

	// DeleteDocument deletes a document
	DeleteDocument(ctx context.Context, doc Document) error

	// Exists checks if a document exists by name
	Exists(ctx context.Context, name string) (bool, error)

	// RegisterDocument assigns a name to a document path
	RegisterDocument(ctx context.Context, name, path string) error
}

var ErrThreadNotFound = fmt.Errorf("thread not found")

// Thread represents a chat thread
type Thread struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Messages  []*llm.Message `json:"messages"`
}

// ThreadRepository is an interface for storing and retrieving chat threads
type ThreadRepository interface {

	// PutThread creates or updates a thread
	PutThread(ctx context.Context, thread *Thread) error

	// GetThread retrieves a thread by ID
	GetThread(ctx context.Context, id string) (*Thread, error)

	// DeleteThread deletes a thread by ID
	DeleteThread(ctx context.Context, id string) error
}

// NewID returns a new UUID
func NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id.String()
}
