package teamconf

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

// HCLTeam represents the top-level HCL structure
type HCLTeam struct {
	Name        string        `hcl:"name,optional"`
	Description string        `hcl:"description,optional"`
	Agents      []AgentConfig `hcl:"agent,block"`
	Tasks       []Task        `hcl:"task,block"`
	Config      Config        `hcl:"config,block"`
	Variables   []HCLVariable `hcl:"variable,block"`
	Tools       []HCLTool     `hcl:"tool,block"`
	Documents   []Document    `hcl:"document,block"`
	Triggers    []Trigger     `hcl:"trigger,block"`
	Schedules   []Schedule    `hcl:"schedule,block"`
	Workflows   []Workflow    `hcl:"workflow,block"`
}

// HCLTool represents a tool definition in HCL
type HCLTool struct {
	Name    string `hcl:"name,label"`
	Enabled bool   `hcl:"enabled,optional"`
}

// HCLVariable represents a variable definition in HCL
type HCLVariable struct {
	Name        string `hcl:"name,label"`
	Type        string `hcl:"type"`
	Description string `hcl:"description,optional"`
	Default     string `hcl:"default,optional"`
}

// VariableValues represents the values for variables
type VariableValues map[string]cty.Value

// LoadHCLDefinition loads an HCL definition from a string
func LoadHCLDefinition(conf []byte, filename string, vars map[string]interface{}) (*HCLTeam, error) {
	parser := hclparse.NewParser()

	// Read and parse the HCL string
	file, diags := parser.ParseHCL([]byte(conf), filename)
	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to parse hcl: %s", diags.Error())
	}

	// Create evaluation context with functions
	evalCtx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"var": cty.ObjectVal(make(map[string]cty.Value)),
		},
		Functions: createStandardFunctions(),
	}

	variableValues := make(VariableValues, len(vars))
	for k, val := range vars {
		switch v := val.(type) {
		case string:
			variableValues[k] = cty.StringVal(v)
		case int:
			variableValues[k] = cty.NumberIntVal(int64(v))
		case float64:
			variableValues[k] = cty.NumberFloatVal(v)
		case bool:
			variableValues[k] = cty.BoolVal(v)
		default:
			return nil, fmt.Errorf("unsupported variable type: %T (key: %s)", v, k)
		}
	}

	// First pass: extract variable definitions
	content, diags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "variable", LabelNames: []string{"name"}},
			{Type: "agent", LabelNames: []string{"name"}},
			{Type: "task", LabelNames: []string{"name"}},
			{Type: "config", LabelNames: []string{}},
			{Type: "tool", LabelNames: []string{"name"}},
			{Type: "document", LabelNames: []string{"name"}},
			{Type: "trigger", LabelNames: []string{"name"}},
			{Type: "schedule", LabelNames: []string{"name"}},
			{Type: "workflow", LabelNames: []string{"name"}},
		},
		Attributes: []hcl.AttributeSchema{
			{Name: "name", Required: false},
			{Name: "description", Required: false},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to extract variable blocks: %s", diags.Error())
	}
	// Create a map to hold variable values
	varValues := make(map[string]cty.Value)

	// Process variable blocks
	for _, block := range content.Blocks {
		if block.Type != "variable" {
			continue
		}
		varName := block.Labels[0]

		// Check if variable value is provided externally
		if value, exists := variableValues[varName]; exists {
			varValues[varName] = value
			continue
		}

		// Try to extract type, description, and default value
		varContent, diags := block.Body.Content(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{Name: "type", Required: true},
				{Name: "description", Required: false},
				{Name: "default", Required: false},
			},
		})
		if diags.HasErrors() {
			return nil, fmt.Errorf("failed to decode variable %s: %s", varName, diags.Error())
		}

		// Process default value if present
		if defaultAttr, exists := varContent.Attributes["default"]; exists {
			val, diags := defaultAttr.Expr.Value(evalCtx)
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to evaluate default value for variable %s: %s", varName, diags.Error())
			}
			varValues[varName] = val
		} else {
			return nil, fmt.Errorf("variable %q is required", varName)
		}
	}

	// Error if any variables were passed that are not defined in the HCL
	for k := range variableValues {
		if _, exists := varValues[k]; !exists {
			return nil, fmt.Errorf("unknown variable provided: %q", k)
		}
	}

	// Update the evaluation context with the variable values
	evalCtx.Variables["var"] = cty.ObjectVal(varValues)

	// Second pass: decode the full configuration with variables and functions
	var def HCLTeam

	// Create maps to store task and agent references
	taskRefs := make(map[string]cty.Value)
	agentRefs := make(map[string]cty.Value)
	toolRefs := make(map[string]cty.Value)
	docRefs := make(map[string]cty.Value)

	// First pass through blocks to collect task, agent, and tool names for references
	for _, block := range content.Blocks {
		switch block.Type {
		case "task":
			taskName := block.Labels[0]
			taskRefs[taskName] = cty.StringVal(taskName)
		case "agent":
			agentName := block.Labels[0]
			agentRefs[agentName] = cty.StringVal(agentName)
		case "tool":
			toolName := block.Labels[0]
			toolRefs[toolName] = cty.StringVal(toolName)
		case "document":
			docName := block.Labels[0]
			docRefs[docName] = cty.StringVal(docName)
		}
	}

	// Add references to evaluation context
	evalCtx.Variables["task"] = cty.ObjectVal(taskRefs)
	evalCtx.Variables["agent"] = cty.ObjectVal(agentRefs)
	evalCtx.Variables["tool"] = cty.ObjectVal(toolRefs)
	evalCtx.Variables["document"] = cty.ObjectVal(docRefs)

	// Use a custom schema to handle blocks with variables
	fullContent, diags := file.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "agent", LabelNames: []string{"name"}},
			{Type: "task", LabelNames: []string{"name"}},
			{Type: "config", LabelNames: []string{}},
			{Type: "variable", LabelNames: []string{"name"}},
			{Type: "tool", LabelNames: []string{"name"}},
			{Type: "document", LabelNames: []string{"name"}},
			{Type: "trigger", LabelNames: []string{"name"}},
			{Type: "schedule", LabelNames: []string{"name"}},
			{Type: "workflow", LabelNames: []string{"name"}},
		},
		Attributes: []hcl.AttributeSchema{
			{Name: "name", Required: false},
			{Name: "description", Required: false},
		},
	})

	if diags.HasErrors() {
		return nil, fmt.Errorf("failed to decode HCL content: %s", diags.Error())
	}

	// Process attributes at the top level
	for name, attr := range fullContent.Attributes {
		switch name {
		case "description":
			value, diags := attr.Expr.Value(evalCtx)
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to evaluate description: %s", diags.Error())
			}
			if value.Type() == cty.String {
				def.Description = value.AsString()
			}
		case "name":
			value, diags := attr.Expr.Value(evalCtx)
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to evaluate name: %s", diags.Error())
			}
			if value.Type() == cty.String {
				def.Name = value.AsString()
			}
		}
	}

	// Process blocks
	for _, block := range fullContent.Blocks {
		switch block.Type {
		case "variable":
			// Skip variables, they were already processed
			continue

		case "config":
			var config Config
			if diags := gohcl.DecodeBody(block.Body, evalCtx, &config); diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode config block: %s", diags.Error())
			}
			def.Config = config

		case "document":
			var doc Document
			doc.Name = block.Labels[0]
			content, diags := block.Body.Content(&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{Name: "id", Required: false},
					{Name: "name", Required: false},
					{Name: "description", Required: false},
					{Name: "path", Required: false},
					{Name: "content", Required: false},
					{Name: "content_type", Required: false},
					{Name: "tags", Required: false},
				},
			})
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode document block: %s", diags.Error())
			}

			// Process each attribute
			for name, attr := range content.Attributes {
				val, diags := attr.Expr.Value(evalCtx)
				if diags.HasErrors() {
					return nil, fmt.Errorf("failed to evaluate %s: %s", name, diags.Error())
				}

				switch name {
				case "id":
					if val.Type() == cty.String {
						doc.ID = val.AsString()
					}
				case "name":
					if val.Type() == cty.String {
						doc.Name = val.AsString()
					}
				case "description":
					if val.Type() == cty.String {
						doc.Description = val.AsString()
					}
				case "path":
					if val.Type() == cty.String {
						doc.Path = val.AsString()
					}
				case "content":
					if val.Type() == cty.String {
						doc.Content = val.AsString()
					}
				case "content_type":
					if val.Type() == cty.String {
						doc.ContentType = val.AsString()
					}
				case "tags":
					if !val.CanIterateElements() {
						return nil, fmt.Errorf("tags must be a list")
					}
					for it := val.ElementIterator(); it.Next(); {
						_, v := it.Element()
						if v.Type() != cty.String {
							return nil, fmt.Errorf("tag must be a string")
						}
						doc.Tags = append(doc.Tags, v.AsString())
					}
				}
			}
			def.Documents = append(def.Documents, doc)

		case "agent":
			var agent AgentConfig
			agent.Name = block.Labels[0]

			// Get the full content first
			content, diags := block.Body.Content(&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{Name: "description", Required: false},
					{Name: "instructions", Required: false},
					{Name: "is_supervisor", Required: false},
					{Name: "subordinates", Required: false},
					{Name: "accepted_events", Required: false},
					{Name: "provider", Required: false},
					{Name: "model", Required: false},
					{Name: "tools", Required: false},
					{Name: "cache_control", Required: false},
					{Name: "max_active_tasks", Required: false},
					{Name: "task_timeout", Required: false},
					{Name: "chat_timeout", Required: false},
					{Name: "tool_iteration_limit", Required: false},
					{Name: "log_level", Required: false},
					{Name: "memory", Required: false},
				},
			})
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode agent block: %s", diags.Error())
			}

			// Process each attribute
			for name, attr := range content.Attributes {
				val, diags := attr.Expr.Value(evalCtx)
				if diags.HasErrors() {
					return nil, fmt.Errorf("failed to evaluate %s: %s", name, diags.Error())
				}

				switch name {
				case "description":
					if val.Type() == cty.String {
						agent.Description = val.AsString()
					}
				case "instructions":
					if val.Type() == cty.String {
						agent.Instructions = val.AsString()
					}
				case "is_supervisor":
					if val.Type() == cty.Bool {
						agent.IsSupervisor = val.True()
					}
				case "subordinates":
					if !val.CanIterateElements() {
						return nil, fmt.Errorf("subordinates must be a list")
					}
					for it := val.ElementIterator(); it.Next(); {
						_, v := it.Element()
						if v.Type() != cty.String {
							return nil, fmt.Errorf("subordinate must be a string")
						}
						agent.Subordinates = append(agent.Subordinates, v.AsString())
					}
				case "accepted_events":
					if !val.CanIterateElements() {
						return nil, fmt.Errorf("accepted_events must be a list")
					}
					for it := val.ElementIterator(); it.Next(); {
						_, v := it.Element()
						if v.Type() != cty.String {
							return nil, fmt.Errorf("accepted event must be a string")
						}
						agent.AcceptedEvents = append(agent.AcceptedEvents, v.AsString())
					}
				case "provider":
					if val.Type() == cty.String {
						agent.Provider = val.AsString()
					}
				case "model":
					if val.Type() == cty.String {
						agent.Model = val.AsString()
					}
				case "tools":
					if !val.CanIterateElements() {
						return nil, fmt.Errorf("tools must be a list")
					}
					for it := val.ElementIterator(); it.Next(); {
						_, v := it.Element()
						if v.Type() != cty.String {
							return nil, fmt.Errorf("tool must be a string")
						}
						agent.Tools = append(agent.Tools, v.AsString())
					}
				case "cache_control":
					if val.Type() == cty.String {
						agent.CacheControl = val.AsString()
					}
				case "chat_timeout":
					if val.Type() == cty.String {
						agent.ChatTimeout = val.AsString()
					}
				case "tool_iteration_limit":
					if val.Type() == cty.Number {
						v, _ := val.AsBigFloat().Int64()
						agent.ToolIterationLimit = int(v)
					}
				case "log_level":
					if val.Type() == cty.String {
						agent.LogLevel = val.AsString()
					}
				case "memory":
					if !val.Type().IsObjectType() {
						return nil, fmt.Errorf("memory must be an object")
					}

					attrs := val.AsValueMap()

					if typeVal, ok := attrs["type"]; ok && typeVal.Type() == cty.String {
						agent.Memory.Type = typeVal.AsString()
					}

					if collectionVal, ok := attrs["collection"]; ok && collectionVal.Type() == cty.String {
						agent.Memory.Collection = collectionVal.AsString()
					}

					if retentionVal, ok := attrs["retention_days"]; ok && retentionVal.Type() == cty.Number {
						retention, _ := retentionVal.AsBigFloat().Int64()
						agent.Memory.RetentionDays = int(retention)
					}
				}
			}

			def.Agents = append(def.Agents, agent)

		case "task":
			var task Task
			task.Name = block.Labels[0]

			// Get the full content first
			content, diags := block.Body.Content(&hcl.BodySchema{
				Attributes: []hcl.AttributeSchema{
					{Name: "description", Required: false},
					{Name: "type", Required: false},
					{Name: "operation", Required: false},
					{Name: "expected_output", Required: false},
					{Name: "output_format", Required: false},
					{Name: "agent", Required: false},
					{Name: "output_file", Required: false},
					{Name: "timeout", Required: false},
				},
				Blocks: []hcl.BlockHeaderSchema{
					{Type: "input", LabelNames: []string{"name"}},
					{Type: "output", LabelNames: []string{"name"}},
				},
			})
			if diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode task block: %s", diags.Error())
			}

			// Process each attribute
			for name, attr := range content.Attributes {
				val, diags := attr.Expr.Value(evalCtx)
				if diags.HasErrors() {
					return nil, fmt.Errorf("failed to evaluate %s: %s", name, diags.Error())
				}

				switch name {
				case "description":
					if val.Type() == cty.String {
						task.Description = val.AsString()
					}
				case "type":
					if val.Type() == cty.String {
						task.Type = val.AsString()
					}
				case "operation":
					if val.Type() == cty.String {
						task.Operation = val.AsString()
					}
				case "expected_output":
					if val.Type() == cty.String {
						task.ExpectedOutput = val.AsString()
					}
				case "output_format":
					if val.Type() == cty.String {
						task.OutputFormat = val.AsString()
					}
				case "agent":
					if val.Type() == cty.String {
						task.Agent = val.AsString()
					}
				case "output_file":
					if val.Type() == cty.String {
						task.OutputFile = val.AsString()
					}
				case "timeout":
					if val.Type() == cty.String {
						task.Timeout = val.AsString()
					}
				}
			}

			// Process input blocks
			task.Inputs = make(map[string]TaskInput)
			for _, block := range content.Blocks {
				if block.Type != "input" {
					continue
				}

				var input TaskInput
				if diags := gohcl.DecodeBody(block.Body, evalCtx, &input); diags.HasErrors() {
					return nil, fmt.Errorf("failed to decode input block: %s", diags.Error())
				}
				task.Inputs[block.Labels[0]] = input
			}

			// Process output blocks
			task.Outputs = make(map[string]TaskOutput)
			for _, block := range content.Blocks {
				if block.Type != "output" {
					continue
				}

				var output TaskOutput
				if diags := gohcl.DecodeBody(block.Body, evalCtx, &output); diags.HasErrors() {
					return nil, fmt.Errorf("failed to decode output block: %s", diags.Error())
				}
				task.Outputs[block.Labels[0]] = output
			}

			def.Tasks = append(def.Tasks, task)

		case "tool":
			var tool HCLTool
			tool.Name = block.Labels[0]
			if diags := gohcl.DecodeBody(block.Body, evalCtx, &tool); diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode tool block: %s", diags.Error())
			}
			// By default, a tool defined in HCL is enabled
			if !tool.Enabled {
				tool.Enabled = true
			}
			def.Tools = append(def.Tools, tool)

		case "trigger":
			var trigger Trigger
			trigger.Name = block.Labels[0]
			if diags := gohcl.DecodeBody(block.Body, evalCtx, &trigger); diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode trigger block: %s", diags.Error())
			}
			def.Triggers = append(def.Triggers, trigger)

		case "schedule":
			var schedule Schedule
			schedule.Name = block.Labels[0]
			if diags := gohcl.DecodeBody(block.Body, evalCtx, &schedule); diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode schedule block: %s", diags.Error())
			}
			def.Schedules = append(def.Schedules, schedule)

		case "workflow":
			var workflow Workflow
			workflow.Name = block.Labels[0]
			if diags := gohcl.DecodeBody(block.Body, evalCtx, &workflow); diags.HasErrors() {
				return nil, fmt.Errorf("failed to decode workflow block: %s", diags.Error())
			}
			def.Workflows = append(def.Workflows, workflow)
			node := workflow.Nodes[0] // "normalize_market_data"]
			fmt.Printf("WORKFLOW NODE: %+v\n", node)

			workflow.Triggers = []string{}

			def.Workflows = append(def.Workflows, workflow)
		}
	}
	return &def, nil
}
