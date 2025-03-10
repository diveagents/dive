package agent

import "github.com/getstingrai/dive"

// agentTemplateData is the data used to render the agent prompt template.
// It carries some information that isn't available via the Agent Go interface.
type agentTemplateData struct {
	*Agent
	DelegateTargets []dive.Agent
}

func newAgentTemplateData(agent *Agent) *agentTemplateData {
	var delegateTargets []dive.Agent
	if agent.isSupervisor {
		if agent.subordinates == nil {
			if agent.environment != nil {
				// Unspecified means we can delegate to all non-supervisors
				for _, a := range agent.environment.Agents() {
					if !a.IsSupervisor() {
						delegateTargets = append(delegateTargets, a)
					}
				}
			}
		} else {
			for _, name := range agent.subordinates {
				other, err := agent.environment.GetAgent(name)
				if err == nil {
					delegateTargets = append(delegateTargets, other)
				}
			}
		}
	}
	return &agentTemplateData{
		Agent:           agent,
		DelegateTargets: delegateTargets,
	}
}
