package workflow

import (
	"fmt"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/graph"
)

// contains checks if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

func AgentNames(agents []dive.Agent) []string {
	var agentNames []string
	for _, agent := range agents {
		agentNames = append(agentNames, agent.Name())
	}
	return agentNames
}

func TaskNames(tasks []dive.Task) []string {
	var taskNames []string
	for _, task := range tasks {
		taskNames = append(taskNames, task.Name())
	}
	return taskNames
}

// orderTasks sorts tasks into execution order using their dependencies.
func orderTasks(tasks []dive.Task) ([]string, error) {
	nodes := make([]graph.Node, len(tasks))
	for i, task := range tasks {
		nodes[i] = task
	}
	order, err := graph.New(nodes).TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("invalid task dependencies: %w", err)
	}
	return order, nil
}
