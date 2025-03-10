package events

// CreateEvent creates a new event with the given parameters
func CreateEvent(eventType string, agentName string, taskName string) *Event {
	return &Event{
		Type:      eventType,
		AgentName: agentName,
		TaskName:  taskName,
	}
}

// CreateStepResult creates a new step result
// func CreateStepResult(step interface{}, content string, object interface{}, usage interface{}) *events.StepResult {
// 	return &events.StepResult{
// 		Step:    step,
// 		Content: content,
// 		Object:  object,
// 		Usage:   usage,
// 	}
// }
