package workflow

import (
	"context"

	"github.com/getstingrai/dive"
)

type Condition interface {
	Evaluate(ctx context.Context, inputs map[string]interface{}) (bool, error)
}

type Variable interface {
	GetValue() any
	SetValue(value any)
}

type Edge struct {
	From      dive.Task
	To        dive.Task
	Condition Condition
}

type Node struct {
	Task    dive.Task
	Inputs  map[string]Variable
	Outputs map[string]Variable
	Next    []*Edge
}

type Graph struct {
	Nodes map[string]*Node
}

func NewGraph(nodes map[string]*Node) *Graph {
	copied := make(map[string]*Node)
	for name, node := range nodes {
		copied[name] = node
	}
	return &Graph{
		Nodes: copied,
	}
}

func (g *Graph) WithNode(node *Node) *Graph {
	g.Nodes[node.Task.Name()] = node
	return g
}
