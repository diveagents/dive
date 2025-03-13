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
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/yaml.v3"
)

type ToolConfig map[string]interface{}

// TeamDefinition is a serializable representation of a dive.Team
type TeamDefinition struct {
	Name        string         `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,optional"`
	Description string         `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Agents      []AgentConfig  `yaml:"agents,omitempty" json:"agents,omitempty" hcl:"agent,block"`
	Tasks       []Task         `yaml:"tasks,omitempty" json:"tasks,omitempty" hcl:"task,block"`
	Documents   []Document     `yaml:"documents,omitempty" json:"documents,omitempty" hcl:"document,block"`
	Variables   map[string]any `yaml:"variables,omitempty" json:"variables,omitempty" hcl:"variable,block"`
	Config      map[string]any `yaml:"config,omitempty" json:"config,omitempty" hcl:"config,block"`
	Tools       []ToolConfig   `yaml:"tools,omitempty" json:"tools,omitempty" hcl:"tool,block"`
	Triggers    []Trigger      `yaml:"triggers,omitempty" json:"triggers,omitempty" hcl:"trigger,block"`
	Schedules   []Schedule     `yaml:"schedules,omitempty" json:"schedules,omitempty" hcl:"schedule,block"`
	Workflows   []Workflow     `yaml:"workflows,omitempty" json:"workflows,omitempty" hcl:"workflow,block"`
}

// AgentMemory represents memory configuration for an agent
type AgentMemory struct {
	Type          string `yaml:"type,omitempty" json:"type,omitempty" hcl:"type"`
	Collection    string `yaml:"collection,omitempty" json:"collection,omitempty" hcl:"collection,optional"`
	RetentionDays int    `yaml:"retention_days,omitempty" json:"retention_days,omitempty" hcl:"retention_days,optional"`
}

// AgentConfig is a serializable representation of a dive.Agent
type AgentConfig struct {
	Name               string         `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description        string         `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Provider           string         `yaml:"provider,omitempty" json:"provider,omitempty" hcl:"provider,optional"`
	Model              string         `yaml:"model,omitempty" json:"model,omitempty" hcl:"model,optional"`
	AcceptedEvents     []string       `yaml:"accepted_events,omitempty" json:"accepted_events,omitempty" hcl:"accepted_events,optional"`
	CacheControl       string         `yaml:"cache_control,omitempty" json:"cache_control,omitempty" hcl:"cache_control,optional"`
	Instructions       string         `yaml:"instructions,omitempty" json:"instructions,omitempty" hcl:"instructions,optional"`
	Tools              []string       `yaml:"tools,omitempty" json:"tools,omitempty" hcl:"tools,optional"`
	IsSupervisor       bool           `yaml:"is_supervisor,omitempty" json:"is_supervisor,omitempty" hcl:"is_supervisor,optional"`
	Subordinates       []string       `yaml:"subordinates,omitempty" json:"subordinates,omitempty" hcl:"subordinates,optional"`
	TaskTimeout        string         `yaml:"task_timeout,omitempty" json:"task_timeout,omitempty" hcl:"task_timeout,optional"`
	ChatTimeout        string         `yaml:"chat_timeout,omitempty" json:"chat_timeout,omitempty" hcl:"chat_timeout,optional"`
	LogLevel           string         `yaml:"log_level,omitempty" json:"log_level,omitempty" hcl:"log_level,optional"`
	ToolConfig         map[string]any `yaml:"tool_config,omitempty" json:"tool_config,omitempty" hcl:"tool_config,block"`
	ToolIterationLimit int            `yaml:"tool_iteration_limit,omitempty" json:"tool_iteration_limit,omitempty" hcl:"tool_iteration_limit,optional"`
	Memory             AgentMemory    `yaml:"memory,omitempty" json:"memory,omitempty" hcl:"memory,block"`
}

// TaskInput represents an input parameter for a task
type TaskInput struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty" hcl:"type"`
	Description string `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty" hcl:"required,optional"`
}

// TaskOutput represents an output parameter for a task
type TaskOutput struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty" hcl:"type"`
	Description string `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
}

// Task is a serializable representation of a dive.Task
type Task struct {
	Name           string                `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description    string                `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Kind           string                `yaml:"kind,omitempty" json:"kind,omitempty" hcl:"kind,optional"`
	Type           string                `yaml:"type,omitempty" json:"type,omitempty" hcl:"type,optional"`
	Operation      string                `yaml:"operation,omitempty" json:"operation,omitempty" hcl:"operation,optional"`
	ExpectedOutput string                `yaml:"expected_output,omitempty" json:"expected_output,omitempty" hcl:"expected_output,optional"`
	OutputFormat   string                `yaml:"output_format,omitempty" json:"output_format,omitempty" hcl:"output_format,optional"`
	Agent          string                `yaml:"agent,omitempty" json:"agent,omitempty" hcl:"agent,optional"`
	OutputFile     string                `yaml:"output_file,omitempty" json:"output_file,omitempty" hcl:"output_file,optional"`
	Timeout        string                `yaml:"timeout,omitempty" json:"timeout,omitempty" hcl:"timeout,optional"`
	Inputs         map[string]TaskInput  `yaml:"inputs,omitempty" json:"inputs,omitempty" hcl:"input,block"`
	Outputs        map[string]TaskOutput `yaml:"outputs,omitempty" json:"outputs,omitempty" hcl:"output,block"`
}

// Trigger represents a trigger definition
type Trigger struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Type string `yaml:"type,omitempty" json:"type,omitempty" hcl:"type"`
	Path string `yaml:"path,omitempty" json:"path,omitempty" hcl:"path,optional"`
	// PayloadSchema map[string]interface{} `yaml:"payload_schema,omitempty" json:"payload_schema,omitempty" hcl:"payload_schema,optional"`
}

// Schedule represents a schedule definition
type Schedule struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Cron string `yaml:"cron,omitempty" json:"cron,omitempty" hcl:"cron"`
	// Payload map[string]interface{} `yaml:"payload,omitempty" json:"payload,omitempty" hcl:"payload,optional"`
}

// type NextNode struct {
// 	Node string
// 	When string
// }

// // Node represents a step in a workflow graph
// type Node struct {
// 	Task   string
// 	Inputs map[string]string
// 	Values map[string]interface{}
// 	Next   []NextNode
// 	When   string
// }

// Node represents a single node in the workflow
type Node struct {
	Name    string    `hcl:"name,label"`
	Task    string    `hcl:"task"`
	Inputs  cty.Value `hcl:"inputs,optional"`
	Next    []string  `hcl:"next,optional"`
	When    string    `hcl:"when,optional"`
	IsStart bool      `hcl:"is_start,optional"`
}

// Workflow represents a workflow definition
type Workflow struct {
	Name        string   `hcl:"name,label"`
	Description string   `hcl:"description,optional"`
	Triggers    []string `hcl:"triggers,optional"`
	Nodes       []*Node  `hcl:"node,block"`
}

// ToolDefinition used for serializing tool configurations
type ToolDefinition map[string]interface{}

// Document represents a document that can be referenced by agents and tasks
type Document struct {
	ID          string   `yaml:"id,omitempty" json:"id,omitempty" hcl:"id,label"`
	Name        string   `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Path        string   `yaml:"path,omitempty" json:"path,omitempty" hcl:"path,optional"`
	Content     string   `yaml:"content,omitempty" json:"content,omitempty" hcl:"content,optional"`
	ContentType string   `yaml:"content_type,omitempty" json:"content_type,omitempty" hcl:"content_type,optional"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty" hcl:"tags,optional"`
}

type Config struct {
	LogLevel        string `yaml:"log_level,omitempty" json:"log_level,omitempty" hcl:"log_level,optional"`
	DefaultProvider string `yaml:"default_provider,omitempty" json:"default_provider,omitempty" hcl:"default_provider,optional"`
	DefaultModel    string `yaml:"default_model,omitempty" json:"default_model,omitempty" hcl:"default_model,optional"`
	CacheControl    string `yaml:"cache_control,omitempty" json:"cache_control,omitempty" hcl:"cache_control,optional"`
}

// LoadFile loads a Team configuration from a file. The file extension is
// used to determine the configuration format:
// - .json -> JSON
// - .yml or .yaml -> YAML
// - .hcl or .dive -> HCL
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
	case ".hcl", ".dive":
		return LoadHCL(data)
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

// LoadHCL loads a Team configuration from HCL data
func LoadHCL(data []byte) (*TeamDefinition, error) {
	// Call LoadHCLDefinition and convert HCLTeam to TeamDefinition
	hclTeam, err := LoadHCLDefinition(data, "config.hcl", nil)
	if err != nil {
		return nil, err
	}

	var tools []ToolConfig
	for _, tool := range hclTeam.Tools {
		tools = append(tools, ToolConfig{
			"name":    tool.Name,
			"enabled": tool.Enabled,
		})
	}

	// Convert HCLTeam to TeamDefinition
	return &TeamDefinition{
		Name:        hclTeam.Name,
		Description: hclTeam.Description,
		Agents:      hclTeam.Agents,
		Tasks:       hclTeam.Tasks,
		Documents:   hclTeam.Documents,
		Config:      map[string]any{}, // Initialize empty map
		Variables:   map[string]any{}, // Initialize empty map
		Tools:       tools,
		Workflows:   hclTeam.Workflows,
	}, nil
}

// LoadDirectory loads all HCL files from a directory and combines them into a single Environment.
// Files are loaded in lexicographical order. Later files can override values from earlier files.
func LoadDirectory(dirPath string) (*environment.Environment, error) {
	// Read all HCL files in the directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Collect all HCL files
	var hclFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".hcl" || ext == ".dive" {
				hclFiles = append(hclFiles, filepath.Join(dirPath, entry.Name()))
			}
		}
	}

	// Sort files for deterministic loading order
	sort.Strings(hclFiles)

	// Load and merge all HCL files
	var combinedDef TeamDefinition
	for _, file := range hclFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		def, err := LoadHCL(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse HCL file %s: %w", file, err)
		}

		// Merge definitions
		combinedDef = mergeDefs(combinedDef, *def)
	}

	// Build environment from combined definition
	buildOpts := BuildOptions{
		Logger:     slogger.New(slogger.LevelFromString("debug")), // combinedDef.Config["log_level"].(string))),
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

	// Merge variables
	if result.Variables == nil {
		result.Variables = make(map[string]any)
	}
	for k, v := range override.Variables {
		result.Variables[k] = v
	}

	// Merge config
	if result.Config == nil {
		result.Config = make(map[string]any)
	}
	for k, v := range override.Config {
		result.Config[k] = v
	}

	// Merge tools
	result.Tools = append(result.Tools, override.Tools...)

	return result
}

// Save writes a Team configuration to a file. The file extension is used to
// determine the configuration format:
// - .json -> JSON
// - .yml or .yaml -> YAML
// - .hcl or .dive -> HCL
func (def *TeamDefinition) Save(path string) error {
	// Determine format from extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return def.SaveJSON(path)
	case ".yml", ".yaml":
		return def.SaveYAML(path)
	case ".hcl", ".dive":
		return def.SaveHCL(path)
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

// SaveHCL writes a Team configuration to an HCL file
func (def *TeamDefinition) SaveHCL(path string) error {
	return fmt.Errorf("not implemented")
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
		if def.Config != nil {
			if logLevelVal, ok := def.Config["log_level"]; ok {
				logLevel = logLevelVal.(string)
			}
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
		for _, toolDef := range def.Tools {
			toolConfigs[toolDef["name"].(string)] = toolDef
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
		if def.Config != nil {
			if v, ok := def.Config["log_level"].(string); ok {
				config.LogLevel = v
			}
			if v, ok := def.Config["default_provider"].(string); ok {
				config.DefaultProvider = v
			}
			if v, ok := def.Config["default_model"].(string); ok {
				config.DefaultModel = v
			}
			if v, ok := def.Config["cache_control"].(string); ok {
				config.CacheControl = v
			}
		}
		agent, err := buildAgent(agentDef, config, toolsMap, buildOpts.Logger, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", agentDef.Name, err)
		}
		agents = append(agents, agent)
	}

	// Create workflows from tasks
	// workflows := make([]*workflow.Workflow, 0, len(def.Tasks))
	var tasks []*workflow.Task
	for _, taskDef := range def.Tasks {
		task, err := buildTask(taskDef, agents, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build task %s: %w", taskDef.Name, err)
		}
		tasks = append(tasks, task)
	}

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
