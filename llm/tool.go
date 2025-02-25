package llm

import (
	"context"
)

type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  Schema `json:"parameters"`
}

func (t *ToolDefinition) ParametersCount() int {
	return len(t.Parameters.Properties)
}

type ToolFunc func(ctx context.Context, input string) (string, error)

type ToolChoice struct {
	Type                   string `json:"type"`
	Name                   string `json:"name,omitempty"`
	DisableParallelToolUse bool   `json:"disable_parallel_tool_use,omitempty"`
}

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
