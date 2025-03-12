package workflow

import (
	"context"
	"fmt"

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
	From      *Node
	To        *Node
	Condition Condition
}

type Node struct {
	Task    dive.Task
	Inputs  map[string]Variable
	Outputs map[string]Variable
	Next    []*Edge
}

func (n *Node) Name() string {
	return n.Task.Name()
}

type Graph struct {
	nodes         map[string]*Node
	startNodeName string
}

func NewGraph(nodes map[string]*Node, start string) (*Graph, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("graph must have at least one node")
	}
	if start == "" {
		return nil, fmt.Errorf("graph must have a start node")
	}
	graphNodes := make(map[string]*Node, len(nodes))
	for name, node := range nodes {
		graphNodes[name] = node
	}
	if _, ok := graphNodes[start]; !ok {
		return nil, fmt.Errorf("start node %q not found", start)
	}
	return &Graph{
		nodes:         graphNodes,
		startNodeName: start,
	}, nil
}

func (g *Graph) StartNode() *Node {
	return g.nodes[g.startNodeName]
}

func (g *Graph) GetNode(name string) (*Node, bool) {
	node, ok := g.nodes[name]
	return node, ok
}

func (g *Graph) NodeNames() []string {
	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	return names
}
