package module

import "github.com/getstingrai/dive/workflow"

type Module struct {
	Name        string
	Description string
	Steps       []*workflow.Step
}

func NewModule(name string, description string) *Module {
	return &Module{Name: name, Description: description}
}
