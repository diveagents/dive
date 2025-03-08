package environment

import (
	"time"

	"github.com/getstingrai/dive"
)

type Task struct {
	name    string
	timeout time.Duration
	prompt  *dive.Prompt
}

func (t *Task) Name() string {
	return t.name
}

func (t *Task) Timeout() time.Duration {
	return t.timeout
}

func (t *Task) Prompt() (*dive.Prompt, error) {
	return t.prompt, nil
}
