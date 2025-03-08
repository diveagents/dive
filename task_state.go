package dive

import (
	"time"

	"github.com/getstingrai/dive/llm"
)

type taskState struct {
	Task               *Task
	Publisher          *StreamPublisher
	Status             TaskStatus
	Iterations         int
	Started            time.Time
	StructuredResponse StructuredResponse
	Messages           []*llm.Message
	Paused             bool
	Usage              llm.Usage
}

func (s *taskState) String() string {
	text, err := executeTemplate(taskStatePromptTemplate, s)
	if err != nil {
		panic(err)
	}
	return text
}

func (s *taskState) TrackResponse(response *llm.Response, updatedMessages []*llm.Message) {
	// Update task state based on the last response from the LLM. It should
	// contain thinking, primary output, then status. We could concatenate
	// the new output with prior output, but for now it seems like it's better
	// not to, and to request a full final response instead.
	taskResponse := ParseStructuredResponse(response.Message().Text())
	// s.Output = taskResponse.Text
	// s.Reasoning = taskResponse.Thinking
	// s.StatusDescription = taskResponse.StatusDescription
	s.Messages = updatedMessages
	s.StructuredResponse = taskResponse
	s.trackUsage(response.Usage())

	// For now, if the status description is empty, let's assume it is complete.
	// We may need to make this configurable in the future.
	if taskResponse.StatusDescription == "" {
		s.Status = TaskStatusCompleted
	} else {
		s.Status = taskResponse.Status()
	}
}

func (s *taskState) trackUsage(usage llm.Usage) {
	s.Usage.InputTokens += usage.InputTokens
	s.Usage.OutputTokens += usage.OutputTokens
	s.Usage.CacheCreationInputTokens += usage.CacheCreationInputTokens
	s.Usage.CacheReadInputTokens += usage.CacheReadInputTokens
}

func (s *taskState) StatusDescription() string {
	return s.StructuredResponse.StatusDescription
}

func (s *taskState) LastOutput() string {
	return s.StructuredResponse.Text
}
