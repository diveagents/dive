package stream

// Event is an event that carries LLM events, task results, or errors.
type Event struct {
	// Type of the event
	Type string

	// Payload carries the data for the event
	Payload any

	// Error conveys an error message
	Error error
}

// // AgentName is the name of the agent associated with the event
// AgentName string `json:"agent_name"`

// // StepName is the name of the step that generated the event (if applicable)
// StepName string `json:"step_name,omitempty"`

// // LLMEvent is the event from the LLM (may be nil)
// LLMEvent *llm.StreamEvent `json:"llm_event,omitempty"`

// // StepResult is the result of a step (may be nil)
// StepResult *StepResult `json:"step_result,omitempty"`

// // Response is the final response from the agent (may be nil)
// Response *llm.Response `json:"response,omitempty"`
