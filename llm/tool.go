package llm

import (
	"context"
	"time"
)

// ToolResult contains the input and output of a tool call
type ToolResult struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Input       string     `json:"input,omitempty"`
	Output      string     `json:"output,omitempty"`
	Error       error      `json:"error,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Schema struct {
	Type       string                     `json:"type"`
	Properties map[string]*SchemaProperty `json:"properties"`
	Required   []string                   `json:"required,omitempty"`
}

type SchemaProperty struct {
	Type        string                     `json:"type"`
	Description string                     `json:"description"`
	Enum        []string                   `json:"enum,omitempty"`
	Items       *SchemaProperty            `json:"items,omitempty"`
	Required    []string                   `json:"required,omitempty"`
	Properties  map[string]*SchemaProperty `json:"properties,omitempty"`
}

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  Schema `json:"parameters"`
}

func (t *ToolDefinition) ParametersCount() int {
	return len(t.Parameters.Properties)
}

type ToolFunc func(ctx context.Context, input string) (string, error)

type ToolChoice string

const (
	ToolChoiceAuto     ToolChoice = "auto"
	ToolChoiceNone     ToolChoice = "none"
	ToolChoiceRequired ToolChoice = "required"
)

type Tool interface {
	Definition() *ToolDefinition
	Call(ctx context.Context, input string) (string, error)
	ShouldReturnResult() bool
}

type StandardTool struct {
	def          *ToolDefinition
	fn           ToolFunc
	returnResult bool
}

func NewTool(def *ToolDefinition, fn ToolFunc) Tool {
	return &StandardTool{
		def:          def,
		fn:           fn,
		returnResult: true,
	}
}

// NewToolWithOptions creates a new tool with additional options
func NewToolWithOptions(def *ToolDefinition, fn ToolFunc, returnResult bool) Tool {
	return &StandardTool{
		def:          def,
		fn:           fn,
		returnResult: returnResult,
	}
}

func (t *StandardTool) Definition() *ToolDefinition {
	return t.def
}

func (t *StandardTool) Call(ctx context.Context, input string) (string, error) {
	return t.fn(ctx, input)
}

func (t *StandardTool) ShouldReturnResult() bool {
	return t.returnResult
}
