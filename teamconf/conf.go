package teamconf

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/environment"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
	"gopkg.in/yaml.v3"
)

// TeamDefinition is a serializable representation of a dive.Team
type TeamDefinition struct {
	Name        string        `yaml:"name,omitempty" json:"name,omitempty"`
	Description string        `yaml:"description,omitempty" json:"description,omitempty"`
	Config      Config        `yaml:"config,omitempty" json:"config,omitempty"`
	Variables   []Variable    `yaml:"variables,omitempty" json:"variables,omitempty"`
	Tools       []Tool        `yaml:"tools,omitempty" json:"tools,omitempty"`
	Documents   []Document    `yaml:"documents,omitempty" json:"documents,omitempty"`
	Agents      []AgentConfig `yaml:"agents,omitempty" json:"agents,omitempty"`
	Tasks       []Task        `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	Workflows   []Workflow    `yaml:"workflows,omitempty" json:"workflows,omitempty"`
	Triggers    []Trigger     `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	Schedules   []Schedule    `yaml:"schedules,omitempty" json:"schedules,omitempty"`
}

// Config represents global configuration settings
type Config struct {
	Logging struct {
		Level string `yaml:"level,omitempty" json:"level,omitempty"`
	} `yaml:"logging,omitempty" json:"logging,omitempty"`
	Providers struct {
		Default string `yaml:"default,omitempty" json:"default,omitempty"`
	} `yaml:"providers,omitempty" json:"providers,omitempty"`
	Model           string `yaml:"model,omitempty" json:"model,omitempty"`
	CacheControl    string `yaml:"cache_control,omitempty" json:"cache_control,omitempty"`
	DefaultProvider string `yaml:"default_provider,omitempty" json:"default_provider,omitempty"`
	DefaultModel    string `yaml:"default_model,omitempty" json:"default_model,omitempty"`
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

// TaskInput represents an input parameter for a task
type TaskInput struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Default     any    `yaml:"default,omitempty" json:"default,omitempty"`
}

// TaskOutput represents an output parameter for a task
type TaskOutput struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Format      string `yaml:"format,omitempty" json:"format,omitempty"`
	Default     any    `yaml:"default,omitempty" json:"default,omitempty"`
}

// Task is a serializable representation of a dive.Task
type Task struct {
	Name        string                `yaml:"name,omitempty" json:"name,omitempty"`
	Description string                `yaml:"description,omitempty" json:"description,omitempty"`
	Kind        string                `yaml:"kind,omitempty" json:"kind,omitempty"`
	Agent       string                `yaml:"agent,omitempty" json:"agent,omitempty"`
	OutputFile  string                `yaml:"output_file,omitempty" json:"output_file,omitempty"`
	Timeout     string                `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Inputs      map[string]TaskInput  `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs     map[string]TaskOutput `yaml:"outputs,omitempty" json:"outputs,omitempty"`
}

// Step represents a single step in a workflow
type Step struct {
	Name   string            `yaml:"name,omitempty" json:"name,omitempty"`
	Task   string            `yaml:"task,omitempty" json:"task,omitempty"`
	Inputs map[string]string `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Each   *EachBlock        `yaml:"each,omitempty" json:"each,omitempty"`
	Next   []NextStep        `yaml:"next,omitempty" json:"next,omitempty"`
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
	Name        string    `yaml:"name,omitempty" json:"name,omitempty"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Triggers    []Trigger `yaml:"triggers,omitempty" json:"triggers,omitempty"`
	Steps       []Step    `yaml:"steps,omitempty" json:"steps,omitempty"`
}

// Trigger represents a trigger definition
type Trigger struct {
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

// LoadFile loads a Team configuration from a file. The file extension is
// used to determine the configuration format:
// - .json -> JSON
// - .yml or .yaml -> YAML
func LoadFile(path string) (*TeamDefinition, error) {
	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Determine format from extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return LoadJSON(data)
	case ".yml", ".yaml":
		return LoadYAML(data)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// LoadYAML loads a Team configuration from YAML data
func LoadYAML(data []byte) (*TeamDefinition, error) {
	var def TeamDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// LoadJSON loads a Team configuration from JSON data
func LoadJSON(data []byte) (*TeamDefinition, error) {
	var def TeamDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	return &def, nil
}

// LoadDirectory loads all YAML files from a directory and combines them into a single Environment.
// Files are loaded in lexicographical order. Later files can override values from earlier files.
func LoadDirectory(dirPath string) (*environment.Environment, error) {
	// Read all YAML files in the directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Collect all YAML files
	var yamlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".yml" || ext == ".yaml" {
				yamlFiles = append(yamlFiles, filepath.Join(dirPath, entry.Name()))
			}
		}
	}

	// Sort files for deterministic loading order
	sort.Strings(yamlFiles)

	// Load and merge all YAML files
	var combinedDef TeamDefinition
	for _, file := range yamlFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		def, err := LoadYAML(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML file %s: %w", file, err)
		}

		// Merge definitions
		combinedDef = mergeDefs(combinedDef, *def)
	}

	// Build environment from combined definition
	buildOpts := BuildOptions{
		Logger:     slogger.New(slogger.LevelFromString("debug")),
		Repository: document.NewMemoryRepository(),
	}

	return combinedDef.BuildEnvironment(buildOpts)
}

// mergeDefs merges two TeamDefinitions, with the second one taking precedence
func mergeDefs(base, override TeamDefinition) TeamDefinition {
	result := base

	// Merge name and description if provided
	if override.Name != "" {
		result.Name = override.Name
	}
	if override.Description != "" {
		result.Description = override.Description
	}

	// Merge config
	result.Config = override.Config

	// Merge variables (by name)
	varMap := make(map[string]Variable)
	for _, v := range result.Variables {
		varMap[v.Name] = v
	}
	for _, v := range override.Variables {
		varMap[v.Name] = v
	}
	result.Variables = make([]Variable, 0, len(varMap))
	for _, v := range varMap {
		result.Variables = append(result.Variables, v)
	}

	// Merge tools (by name)
	toolMap := make(map[string]Tool)
	for _, t := range result.Tools {
		toolMap[t.Name] = t
	}
	for _, t := range override.Tools {
		toolMap[t.Name] = t
	}
	result.Tools = make([]Tool, 0, len(toolMap))
	for _, t := range toolMap {
		result.Tools = append(result.Tools, t)
	}

	// Merge agents (by name)
	agentMap := make(map[string]AgentConfig)
	for _, agent := range result.Agents {
		agentMap[agent.Name] = agent
	}
	for _, agent := range override.Agents {
		agentMap[agent.Name] = agent
	}
	result.Agents = make([]AgentConfig, 0, len(agentMap))
	for _, agent := range agentMap {
		result.Agents = append(result.Agents, agent)
	}

	// Merge tasks (by name)
	taskMap := make(map[string]Task)
	for _, task := range result.Tasks {
		taskMap[task.Name] = task
	}
	for _, task := range override.Tasks {
		taskMap[task.Name] = task
	}
	result.Tasks = make([]Task, 0, len(taskMap))
	for _, task := range taskMap {
		result.Tasks = append(result.Tasks, task)
	}

	// Merge workflows (by name)
	workflowMap := make(map[string]Workflow)
	for _, workflow := range result.Workflows {
		workflowMap[workflow.Name] = workflow
	}
	for _, workflow := range override.Workflows {
		workflowMap[workflow.Name] = workflow
	}
	result.Workflows = make([]Workflow, 0, len(workflowMap))
	for _, workflow := range workflowMap {
		result.Workflows = append(result.Workflows, workflow)
	}

	// Merge documents (by ID)
	docMap := make(map[string]Document)
	for _, doc := range result.Documents {
		docMap[doc.ID] = doc
	}
	for _, doc := range override.Documents {
		docMap[doc.ID] = doc
	}
	result.Documents = make([]Document, 0, len(docMap))
	for _, doc := range docMap {
		result.Documents = append(result.Documents, doc)
	}

	// Merge triggers (by name)
	triggerMap := make(map[string]Trigger)
	for _, trigger := range result.Triggers {
		triggerMap[trigger.Type] = trigger
	}
	for _, trigger := range override.Triggers {
		triggerMap[trigger.Type] = trigger
	}
	result.Triggers = make([]Trigger, 0, len(triggerMap))
	for _, trigger := range triggerMap {
		result.Triggers = append(result.Triggers, trigger)
	}

	// Merge schedules (by name)
	scheduleMap := make(map[string]Schedule)
	for _, schedule := range result.Schedules {
		scheduleMap[schedule.Name] = schedule
	}
	for _, schedule := range override.Schedules {
		scheduleMap[schedule.Name] = schedule
	}
	result.Schedules = make([]Schedule, 0, len(scheduleMap))
	for _, schedule := range scheduleMap {
		result.Schedules = append(result.Schedules, schedule)
	}

	return result
}

// Save writes a Team configuration to a file. The file extension is used to
// determine the configuration format:
// - .json -> JSON
// - .yml or .yaml -> YAML
func (def *TeamDefinition) Save(path string) error {
	// Determine format from extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return def.SaveJSON(path)
	case ".yml", ".yaml":
		return def.SaveYAML(path)
	default:
		return fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// SaveYAML writes a Team configuration to a YAML file
func (def *TeamDefinition) SaveYAML(path string) error {
	data, err := yaml.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SaveJSON writes a Team configuration to a JSON file
func (def *TeamDefinition) SaveJSON(path string) error {
	data, err := yaml.Marshal(def)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Build creates a new Environment from the configuration
func (def *TeamDefinition) Build(opts ...BuildOption) (*environment.Environment, error) {
	buildOpts := &BuildOptions{}
	for _, opt := range opts {
		opt(buildOpts)
	}

	// Set default logger if not provided
	if buildOpts.Logger == nil {
		logLevel := "info"
		if def.Config.Logging.Level != "" {
			logLevel = def.Config.Logging.Level
		}
		buildOpts.Logger = slogger.New(slogger.LevelFromString(logLevel))
	}

	return def.BuildEnvironment(*buildOpts)
}

// Write writes a Team configuration to a writer in YAML format
func (def *TeamDefinition) Write(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(def)
}

// BuildEnvironment creates a new Environment from the configuration
func (def *TeamDefinition) BuildEnvironment(buildOpts BuildOptions) (*environment.Environment, error) {
	// Initialize tools
	var toolConfigs map[string]map[string]interface{}
	if def.Tools != nil {
		toolConfigs = make(map[string]map[string]interface{}, len(def.Tools))
		for _, tool := range def.Tools {
			toolConfigs[tool.Name] = map[string]interface{}{
				"enabled": tool.Enabled,
			}
		}
	}

	toolsMap, err := initializeTools(toolConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools: %w", err)
	}

	// Create agents
	agents := make([]dive.Agent, 0, len(def.Agents))
	for _, agentDef := range def.Agents {
		config := Config{}
		if def.Config.Logging.Level != "" {
			config.Logging.Level = def.Config.Logging.Level
		}
		if def.Config.Providers.Default != "" {
			config.Providers.Default = def.Config.Providers.Default
		}
		if def.Config.Model != "" {
			config.Model = def.Config.Model
		}
		if def.Config.CacheControl != "" {
			config.CacheControl = def.Config.CacheControl
		}
		agent, err := buildAgent(agentDef, config, toolsMap, buildOpts.Logger, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", agentDef.Name, err)
		}
		agents = append(agents, agent)
	}

	// Create tasks
	var tasks []*workflow.Task
	for _, taskDef := range def.Tasks {
		task, err := buildTask(taskDef, agents, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build task %s: %w", taskDef.Name, err)
		}
		tasks = append(tasks, task)
	}

	// Create workflows
	var workflows []*workflow.Workflow
	for _, workflowDef := range def.Workflows {
		workflow, err := buildWorkflow(workflowDef, tasks)
		if err != nil {
			return nil, fmt.Errorf("failed to build workflow %s: %w", workflowDef.Name, err)
		}
		workflows = append(workflows, workflow)
	}

	// Handle documents if repository is provided
	if len(def.Documents) > 0 && buildOpts.Repository == nil {
		return nil, fmt.Errorf("repository is required when documents are defined")
	}

	if buildOpts.Repository != nil {
		for _, docRef := range def.Documents {
			if _, err := ResolveDocument(context.Background(), buildOpts.Repository, docRef); err != nil {
				return nil, fmt.Errorf("failed to resolve document %s: %w", docRef.Name, err)
			}
		}
	}

	// Create environment
	env, err := environment.New(environment.EnvironmentOptions{
		Name:        def.Name,
		Description: def.Description,
		Agents:      agents,
		Workflows:   workflows,
		Logger:      buildOpts.Logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create environment: %w", err)
	}

	return env, nil
}

// ResolveDocument resolves a document reference into a concrete document
func ResolveDocument(ctx context.Context, store document.Repository, ref Document) (document.Document, error) {
	docPath := ref.Path
	if docPath == "" && ref.Name != "" {
		docPath = "./" + ref.Name
	}
	if docPath == "" {
		return nil, fmt.Errorf("document %q must have either path or name", ref.Name)
	}

	// Create document options with resolved path
	docOpts := document.DocumentOptions{
		ID:          ref.ID,
		Name:        ref.Name,
		Description: ref.Description,
		Path:        docPath,
		ContentType: ref.ContentType,
		Tags:        ref.Tags,
	}

	// If content is provided, create/update the document with that content
	if ref.Content != "" {
		docOpts.Content = ref.Content
		doc := document.NewTextDocument(docOpts)
		// Store the document with its content
		if err := store.PutDocument(ctx, doc); err != nil {
			return nil, fmt.Errorf("failed to store document %q: %w", ref.Name, err)
		}
		return doc, nil
	}

	// Try to get existing document
	existingDoc, err := store.GetDocument(ctx, docPath)
	if err == nil {
		// Document exists, return it
		return existingDoc, nil
	}

	// Document doesn't exist, create an empty one
	doc := document.NewTextDocument(docOpts)
	if err := store.PutDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to create empty document %q: %w", ref.Name, err)
	}
	return doc, nil
}
