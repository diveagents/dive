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
	To        string
	Condition Condition
}

type EachBlock struct {
	Array         string
	As            string
	Parallel      bool
	MaxConcurrent int
}

type Node struct {
	name    string
	task    dive.Task
	inputs  map[string]interface{}
	outputs map[string]interface{}
	next    []*Edge
	isStart bool
	each    *EachBlock
}

type NodeOptions struct {
	Name    string
	Task    dive.Task
	Inputs  map[string]interface{}
	Outputs map[string]interface{}
	Next    []*Edge
	IsStart bool
	Each    *EachBlock
}

func NewNode(opts NodeOptions) *Node {
	return &Node{
		name:    opts.Name,
		task:    opts.Task,
		inputs:  opts.Inputs,
		outputs: opts.Outputs,
		next:    opts.Next,
		isStart: opts.IsStart,
		each:    opts.Each,
	}
}

func (n *Node) IsStart() bool {
	return n.isStart
}

func (n *Node) Name() string {
	return n.name
}

func (n *Node) Task() dive.Task {
	return n.task
}

func (n *Node) TaskName() string {
	return n.task.Name()
}

func (n *Node) Inputs() map[string]interface{} {
	return n.inputs
}

func (n *Node) Outputs() map[string]interface{} {
	return n.outputs
}

func (n *Node) Next() []*Edge {
	return n.next
}

func (n *Node) Each() *EachBlock {
	return n.each
}

type Graph struct {
	nodes map[string]*Node
	start []*Node
}

type GraphOptions struct {
	Nodes map[string]*Node
}

// NewGraph creates a new graph containing given nodes
func NewGraph(opts GraphOptions) *Graph {
	if len(opts.Nodes) == 1 {
		for _, node := range opts.Nodes {
			node.isStart = true
			break
		}
	}
	var startNodes []*Node
	graphNodes := make(map[string]*Node, len(opts.Nodes))
	for name, node := range opts.Nodes {
		if node.name == "" {
			node.name = name
		}
		if node.isStart {
			startNodes = append(startNodes, node)
		}
		graphNodes[name] = node
	}
	return &Graph{
		nodes: graphNodes,
		start: startNodes,
	}
}

// Start returns the start node(s) of the graph
func (g *Graph) Start() []*Node {
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

func (g *Graph) Validate() error {
	if len(g.nodes) == 0 {
		return fmt.Errorf("graph must have at least one node")
	}
	for _, node := range g.nodes {
		if node.name == "" {
			return fmt.Errorf("node name cannot be empty")
		}
		for _, edge := range node.next {
			if _, ok := g.nodes[edge.To]; !ok {
				return fmt.Errorf("edge to node %q not found", edge.To)
			}
		}
	}
	return nil
}
