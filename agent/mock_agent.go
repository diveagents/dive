package agent

import (
	"context"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/llm"
)

// WorkFunc is a function that returns a dive.EventStream.
// type WorkFunc func(ctx context.Context, task dive.Task) (dive.EventStream, error)

type MockAgentOptions struct {
	Name         string
	Goal         string
	Backstory    string
	IsSupervisor bool
	Subordinates []string
	// Work           WorkFunc
	AcceptedEvents []string
	Response       *llm.Response
}

type MockAgent struct {
	name         string
	goal         string
	backstory    string
	isSupervisor bool
	subordinates []string
	environment  dive.Environment
	// work           WorkFunc
	acceptedEvents []string
	response       *llm.Response
}

func NewMockAgent(opts MockAgentOptions) *MockAgent {
	return &MockAgent{
		name:           opts.Name,
		goal:           opts.Goal,
		backstory:      opts.Backstory,
		isSupervisor:   opts.IsSupervisor,
		subordinates:   opts.Subordinates,
		acceptedEvents: opts.AcceptedEvents,
		response:       opts.Response,
	}
}

func (a *MockAgent) Name() string {
	return a.name
}

func (a *MockAgent) Goal() string {
	return a.goal
}

func (a *MockAgent) Backstory() string {
	return a.backstory
}

func (a *MockAgent) IsSupervisor() bool {
	return a.isSupervisor
}

func (a *MockAgent) SetEnvironment(env dive.Environment) error {
	a.environment = env
	return nil
}

func (a *MockAgent) CreateResponse(ctx context.Context, opts ...dive.ChatOption) (*dive.Response, error) {
	return nil, nil
}

func (a *MockAgent) StreamResponse(ctx context.Context, opts ...dive.ChatOption) (dive.ResponseStream, error) {
	stream, publisher := dive.NewEventStream()
	publisher.Send(ctx, &dive.Event{
		Type:    "llm.response",
		Payload: a.response,
	})
	return stream, nil
}
