package environment

import (
	"context"
	"fmt"

	"github.com/getstingrai/dive"
)

func TaskNames(tasks []dive.Task) []string {
	var taskNames []string
	for _, task := range tasks {
		taskNames = append(taskNames, task.Name())
	}
	return taskNames
}

func executeTask(ctx context.Context, agent dive.Agent, task dive.Task) (*dive.TaskResult, error) {
	iterator, err := agent.Work(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to start task %q: %w", task.Name(), err)
	}
	defer iterator.Close()

	taskResult, err := dive.WaitForEvent[*dive.TaskResult](ctx, iterator)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for task result: %w", err)
	}
	return taskResult, nil
}
