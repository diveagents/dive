package agent

import (
	"context"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/events"
)

type WorkFunc func(ctx context.Context, task dive.Task) (events.Stream, error)

type HandleEventFunc func(ctx context.Context, event *events.Event) error

type MockAgentOptions struct {
	Name           string
	Description    string
	Instructions   string
	IsSupervisor   bool
	Subordinates   []string
	Work           WorkFunc
	AcceptedEvents []string
	HandleEvent    HandleEventFunc
}

type MockAgent struct {
	name           string
	description    string
	instructions   string
	isSupervisor   bool
	subordinates   []string
	environment    dive.Environment
	work           WorkFunc
	acceptedEvents []string
	handleEvent    HandleEventFunc
}

func NewMockAgent(opts MockAgentOptions) *MockAgent {
	return &MockAgent{
		name:           opts.Name,
		description:    opts.Description,
		instructions:   opts.Instructions,
		isSupervisor:   opts.IsSupervisor,
		subordinates:   opts.Subordinates,
		work:           opts.Work,
		acceptedEvents: opts.AcceptedEvents,
		handleEvent:    opts.HandleEvent,
	}
}

func (a *MockAgent) Name() string {
	return a.name
}

func (a *MockAgent) Description() string {
	return a.description
}

func (a *MockAgent) Instructions() string {
	return a.instructions
}

func (a *MockAgent) AcceptedEvents() []string {
	return a.acceptedEvents
}

func (a *MockAgent) HandleEvent(ctx context.Context, event *events.Event) error {
	return a.handleEvent(ctx, event)
}

func (a *MockAgent) IsSupervisor() bool {
	return a.isSupervisor
}

func (a *MockAgent) Subordinates() []string {
	return a.subordinates
}

func (a *MockAgent) SetEnvironment(env dive.Environment) {
	a.environment = env
}

func (a *MockAgent) Work(ctx context.Context, task dive.Task) (events.Stream, error) {
	return a.work(ctx, task)
}

func (a *MockAgent) Start(ctx context.Context) error {
	return nil
}

func (a *MockAgent) Stop(ctx context.Context) error {
	return nil
}

func (a *MockAgent) IsRunning() bool {
	return true
}
