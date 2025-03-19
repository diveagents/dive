package config

// Config represents global configuration settings
type Config struct {
	CacheControl    string `yaml:"cache_control,omitempty" json:"cache_control,omitempty"`
	DefaultProvider string `yaml:"default_provider,omitempty" json:"default_provider,omitempty"`
	DefaultModel    string `yaml:"default_model,omitempty" json:"default_model,omitempty"`
	DefaultWorkflow string `yaml:"default_workflow,omitempty" json:"default_workflow,omitempty"`
	LogLevel        string `yaml:"log_level,omitempty" json:"log_level,omitempty"`
}

// Variable represents a workflow-level input parameter
type Variable struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Tool represents an external capability that can be used by agents
type Tool struct {
	Name    string `yaml:"name,omitempty" json:"name,omitempty"`
	Enabled bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// AgentMemory represents memory configuration for an agent
type AgentMemory struct {
	Type          string `yaml:"type,omitempty" json:"type,omitempty"`
	Collection    string `yaml:"collection,omitempty" json:"collection,omitempty"`
	RetentionDays int    `yaml:"retention_days,omitempty" json:"retention_days,omitempty"`
}

// AgentConfig is a serializable representation of a dive.Agent
type AgentConfig struct {
	Name               string         `yaml:"name,omitempty" json:"name,omitempty"`
	Description        string         `yaml:"description,omitempty" json:"description,omitempty"`
	Provider           string         `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model              string         `yaml:"model,omitempty" json:"model,omitempty"`
	AcceptedEvents     []string       `yaml:"accepted_events,omitempty" json:"accepted_events,omitempty"`
	CacheControl       string         `yaml:"cache_control,omitempty" json:"cache_control,omitempty"`
	Instructions       string         `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Tools              []string       `yaml:"tools,omitempty" json:"tools,omitempty"`
	IsSupervisor       bool           `yaml:"is_supervisor,omitempty" json:"is_supervisor,omitempty"`
	Subordinates       []string       `yaml:"subordinates,omitempty" json:"subordinates,omitempty"`
	TaskTimeout        string         `yaml:"task_timeout,omitempty" json:"task_timeout,omitempty"`
	ChatTimeout        string         `yaml:"chat_timeout,omitempty" json:"chat_timeout,omitempty"`
	LogLevel           string         `yaml:"log_level,omitempty" json:"log_level,omitempty"`
	ToolConfig         map[string]any `yaml:"tool_config,omitempty" json:"tool_config,omitempty"`
	ToolIterationLimit int            `yaml:"tool_iteration_limit,omitempty" json:"tool_iteration_limit,omitempty"`
	Memory             AgentMemory    `yaml:"memory,omitempty" json:"memory,omitempty"`
}

// Input represents an input parameter for a task or workflow
type Input struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default     any    `yaml:"default,omitempty" json:"default,omitempty"`
}

// Output represents an output parameter for a task or workflow
type Output struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Format      string `yaml:"format,omitempty" json:"format,omitempty"`
	Default     any    `yaml:"default,omitempty" json:"default,omitempty"`
}

// Task is a serializable representation of a dive.Task
type Task struct {
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Kind        string            `yaml:"kind,omitempty" json:"kind,omitempty"`
	Agent       string            `yaml:"agent,omitempty" json:"agent,omitempty"`
	OutputFile  string            `yaml:"output_file,omitempty" json:"output_file,omitempty"`
	Timeout     string            `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Inputs      map[string]Input  `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs     map[string]Output `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

// Step represents a single step in a workflow
type Step struct {
	Name    string         `yaml:"name,omitempty" json:"name,omitempty"`
	Task    string         `yaml:"task,omitempty" json:"task,omitempty"`
	With    map[string]any `yaml:"with,omitempty" json:"with,omitempty"`
	Each    *EachBlock     `yaml:"each,omitempty" json:"each,omitempty"`
	Next    []NextStep     `yaml:"next,omitempty" json:"next,omitempty"`
	IsStart bool           `yaml:"is_start,omitempty" json:"is_start,omitempty"`
}

// EachBlock represents iteration configuration for a step
type EachBlock struct {
	Array         string `yaml:"array,omitempty" json:"array,omitempty"`
	As            string `yaml:"as,omitempty" json:"as,omitempty"`
	Parallel      bool   `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	MaxConcurrent int    `yaml:"max_concurrent,omitempty" json:"max_concurrent,omitempty"`
}

// NextStep represents the next step in a workflow with optional conditions
type NextStep struct {
	Node      string `yaml:"node,omitempty" json:"node,omitempty"`
	Condition string `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// Workflow represents a workflow definition
type Workflow struct {
	Name        string            `yaml:"name,omitempty" json:"name,omitempty"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Inputs      map[string]Input  `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs     map[string]Output `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Triggers    []Trigger         `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	Steps       []Step            `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// Trigger represents a trigger definition
type Trigger struct {
	Name   string                 `yaml:"name,omitempty" json:"name,omitempty"`
	Type   string                 `yaml:"type,omitempty" json:"type,omitempty"`
	Config map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
}

// Schedule represents a schedule definition
type Schedule struct {
	Name     string `yaml:"name,omitempty" json:"name,omitempty"`
	Cron     string `yaml:"cron,omitempty" json:"cron,omitempty"`
	Workflow string `yaml:"workflow,omitempty" json:"workflow,omitempty"`
	Enabled  bool   `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// Document represents a document that can be referenced by agents and tasks
type Document struct {
	ID          string   `yaml:"id,omitempty" json:"id,omitempty"`
	Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Path        string   `yaml:"path,omitempty" json:"path,omitempty"`
	Content     string   `yaml:"content,omitempty" json:"content,omitempty"`
	ContentType string   `yaml:"content_type,omitempty" json:"content_type,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}
