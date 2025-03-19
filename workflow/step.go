package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/getstingrai/dive"
	"github.com/risor-io/risor/compiler"
)

type Condition interface {
	Evaluate(ctx context.Context, inputs map[string]interface{}) (bool, error)
}

type Variable interface {
	GetValue() any
	SetValue(value any)
}

type Edge struct {
	To        string
	Condition Condition
}

type EachBlock struct {
	Array         any
	As            string
	Parallel      bool
	MaxConcurrent int
}

type Step struct {
	name     string
	task     dive.Task
	with     map[string]any
	withCode map[string]*compiler.Code
	next     []*Edge
	isStart  bool
	each     *EachBlock
}

type StepOptions struct {
	Name    string
	Task    dive.Task
	With    map[string]any
	Next    []*Edge
	IsStart bool
	Each    *EachBlock
}

func NewStep(opts StepOptions) *Step {
	return &Step{
		name:    opts.Name,
		task:    opts.Task,
		with:    opts.With,
		next:    opts.Next,
		isStart: opts.IsStart,
		each:    opts.Each,
	}
}

func (s *Step) IsStart() bool {
	return s.isStart
}

func (s *Step) SetIsStart(isStart bool) {
	s.isStart = isStart
}

func (s *Step) Name() string {
	return s.name
}

func (s *Step) Task() dive.Task {
	return s.task
}

func (s *Step) TaskName() string {
	return s.task.Name()
}

func (s *Step) With() map[string]any {
	return s.with
}

func (s *Step) Next() []*Edge {
	return s.next
}

func (s *Step) Each() *EachBlock {
	return s.each
}

func (s *Step) Compile(ctx context.Context) error {
	if s.withCode != nil {
		return nil // already compiled
	}
	withCode := make(map[string]*compiler.Code)
	for name, value := range s.with {
		strValue, ok := value.(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(strValue, "$(") {
			continue
		}
		if !strings.HasSuffix(strValue, ")") {
			return fmt.Errorf("step: %q variable: %q error: invalid with value", s.name, name)
		}
		// Extract and compile the code
		trimmedValue := strings.TrimPrefix(strValue, "$(")
		trimmedValue = strings.TrimSuffix(trimmedValue, ")")
		code, err := compileScript(ctx, trimmedValue, map[string]any{
			"inputs": nil,
		})
		if err != nil {
			return fmt.Errorf("step: %q variable: %q error: %w", s.name, name, err)
		}
		withCode[name] = code
	}
	s.withCode = withCode
	return nil
}

func (s *Step) Code(name string) (*compiler.Code, bool) {
	code, ok := s.withCode[name]
	return code, ok
}
