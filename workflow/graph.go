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
	nodes map[string]*Node
	start *Node
}

type GraphOptions struct {
	Nodes map[string]*Node
	Start string
}

// NewGraph creates a new graph containing given nodes
func NewGraph(opts GraphOptions) (*Graph, error) {
	if len(opts.Nodes) == 0 {
		return nil, fmt.Errorf("graph must have at least one node")
	}
	if len(opts.Nodes) == 1 && opts.Start == "" {
		for name := range opts.Nodes {
			opts.Start = name
			break
		}
	}
	if opts.Start == "" {
		return nil, fmt.Errorf("graph must have a start node")
	}
	graphNodes := make(map[string]*Node, len(opts.Nodes))
	for name, node := range opts.Nodes {
		graphNodes[name] = node
	}
	startNode, ok := graphNodes[opts.Start]
	if !ok {
		return nil, fmt.Errorf("start node %q not found", opts.Start)
	}
	return &Graph{
		nodes: graphNodes,
		start: startNode,
	}, nil
}

// Start returns the start node of the graph
func (g *Graph) Start() *Node {
	return g.start
}

// Get returns a node by name
func (g *Graph) Get(name string) (*Node, bool) {
	node, ok := g.nodes[name]
	return node, ok
}

// Names returns the names of all nodes in the graph
func (g *Graph) Names() []string {
	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	return names
}
