package workflow

import (
	"github.com/getstingrai/dive"
)

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
