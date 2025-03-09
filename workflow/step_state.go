package workflow

import (
	"time"

	"github.com/getstingrai/dive/llm"
	"github.com/getstingrai/dive/stream"
)

type stepState struct {
	Step               *Step
	Publisher          *stream.Publisher
	Status             StepStatus
	Iterations         int
	Started            time.Time
	StructuredResponse StructuredResponse
	Messages           []*llm.Message
	Paused             bool
	Usage              llm.Usage
}

func (s *stepState) String() string {
	text, err := executeTemplate(stepStatePromptTemplate, s)
	if err != nil {
		panic(err)
	}
	return text
}

func (s *stepState) TrackResponse(response *llm.Response, updatedMessages []*llm.Message) {
	// Update task state based on the last response from the LLM. It should
	// contain thinking, primary output, then status. We could concatenate
	// the new output with prior output, but for now it seems like it's better
	// not to, and to request a full final response instead.
	stepResponse := ParseStructuredResponse(response.Message().Text())
	// s.Output = taskResponse.Text
	// s.Reasoning = taskResponse.Thinking
	// s.StatusDescription = taskResponse.StatusDescription
	s.Messages = updatedMessages
	s.StructuredResponse = stepResponse
	s.trackUsage(response.Usage())

	// For now, if the status description is empty, let's assume it is complete.
	// We may need to make this configurable in the future.
	if stepResponse.StatusDescription == "" {
		s.Status = StepStatusCompleted
	} else {
		s.Status = stepResponse.Status()
	}
}

func (s *stepState) trackUsage(usage llm.Usage) {
	s.Usage.InputTokens += usage.InputTokens
	s.Usage.OutputTokens += usage.OutputTokens
	s.Usage.CacheCreationInputTokens += usage.CacheCreationInputTokens
	s.Usage.CacheReadInputTokens += usage.CacheReadInputTokens
}

func (s *stepState) StatusDescription() string {
	return s.StructuredResponse.StatusDescription
}

func (s *stepState) LastOutput() string {
	return s.StructuredResponse.Text
}
