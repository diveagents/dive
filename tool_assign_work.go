package agents

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/getstingrai/agents/llm"
)

var _ llm.Tool = &AssignWorkTool{}

type AssignWorkInput struct {
	AgentName      string `json:"agent"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	ExpectedOutput string `json:"expected_output"`
	OutputFormat   string `json:"output_format"`
	Context        string `json:"context"`
}

type AssignWorkTool struct {
	agent Agent
}

func NewAssignWorkTool(agent Agent) *AssignWorkTool {
	return &AssignWorkTool{agent: agent}
}

func (t *AssignWorkTool) Name() string {
	return "AssignWork"
}

func (t *AssignWorkTool) Description() string {
	return "Assigns work to another team member. Provide a complete and detailed request for the agent to fulfill. It will respond with the result of the request."
}

func (t *AssignWorkTool) Definition() *llm.ToolDefinition {
	return &llm.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: llm.Schema{
			Type: "object",
			Required: []string{
				"agent",
				"name",
				"description",
				"expected_output",
			},
			Properties: map[string]*llm.SchemaProperty{
				"agent": {
					Type:        "string",
					Description: "The name of the agent that should do the work.",
				},
				"name": {
					Type:        "string",
					Description: "The name of the job to be done (e.g. 'Research Company Reviews').",
				},
				"description": {
					Type:        "string",
					Description: "The complete description of the job to be done (e.g. 'Find reviews for a company online').",
				},
				"expected_output": {
					Type:        "string",
					Description: "What the output of the work should look like (e.g. a list of URLs, a list of companies, etc.)",
				},
				"output_format": {
					Type:        "string",
					Description: "The desired output format: text, markdown, or json (optional).",
				},
				"context": {
					Type:        "string",
					Description: "Any additional context that may be relevant and aid the agent in completing the work (optional).",
				},
			},
		},
	}
}

func (t *AssignWorkTool) Call(ctx context.Context, input string) (string, error) {
	var params AssignWorkInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", err
	}
	if params.AgentName == "" {
		return "", fmt.Errorf("agent name is required")
	}
	if params.Name == "" {
		return "", fmt.Errorf("name is required")
	}
	if params.Description == "" {
		return "", fmt.Errorf("description is required")
	}
	if params.ExpectedOutput == "" {
		return "", fmt.Errorf("expected output is required")
	}
	if params.AgentName == t.agent.Name() {
		return "", fmt.Errorf("cannot delegate task to self")
	}
	agent, ok := t.agent.Team().GetAgent(params.AgentName)
	if !ok {
		return "", fmt.Errorf("agent not found")
	}
	outputFormat := OutputFormat(params.OutputFormat)
	if outputFormat == "" {
		outputFormat = OutputMarkdown
	}
	// Generate a dynamic task for the agent
	task := NewTask(TaskSpec{
		Name:           params.Name,
		Description:    params.Description,
		ExpectedOutput: params.ExpectedOutput,
		Context:        params.Context,
		OutputFormat:   outputFormat,
	})
	promise, err := agent.Work(ctx, task)
	if err != nil {
		return fmt.Sprintf("I couldn't complete this work due to the following error: %s", err.Error()), nil
	}
	result, err := promise.Get(ctx)
	if err != nil {
		return fmt.Sprintf("I couldn't complete this work due to the following error: %s", err.Error()), nil
	}
	return result.Output.Content, nil
}
