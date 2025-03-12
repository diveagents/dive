package environment

import "sync"

type ExecutionHandle struct {
	execution *Execution
	wg        sync.WaitGroup
}

func (h *ExecutionHandle) ID() string {
	return h.execution.ID()
}

func (h *ExecutionHandle) Wait() error {
	h.wg.Wait()
	return h.execution.err
}
