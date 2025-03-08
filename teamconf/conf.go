package teamconf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/slogger"
	"gopkg.in/yaml.v3"
)

// Team is a serializable representation of a dive.Team
type Team struct {
	Name        string           `yaml:"name,omitempty" json:"name,omitempty"`
	Description string           `yaml:"description,omitempty" json:"description,omitempty"`
	Agents      []Agent          `yaml:"agents,omitempty" json:"agents,omitempty"`
	Tasks       []Task           `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	Tools       []ToolDefinition `yaml:"tools,omitempty" json:"tools,omitempty"`
	Config      Config           `yaml:"config,omitempty" json:"config,omitempty"`
	Variables   []Variable       `yaml:"variables,omitempty" json:"variables,omitempty"`
	Documents   []Document       `yaml:"documents,omitempty" json:"documents,omitempty"`
}

// Config is a serializable high-level configuration for Dive
type Config struct {
	DefaultProvider string `yaml:"default_provider,omitempty" json:"default_provider,omitempty" hcl:"default_provider,optional"`
	DefaultModel    string `yaml:"default_model,omitempty" json:"default_model,omitempty" hcl:"default_model,optional"`
	LogLevel        string `yaml:"log_level,omitempty" json:"log_level,omitempty" hcl:"log_level,optional"`
	CacheControl    string `yaml:"cache_control,omitempty" json:"cache_control,omitempty" hcl:"cache_control,optional"`
	OutputDir       string `yaml:"output_dir,omitempty" json:"output_dir,omitempty" hcl:"output_dir,optional"`
}

// Variable is used to dynamically configure a dive.Team
type Variable struct {
	Name        string `yaml:"name,omitempty" json:"name,omitempty"`
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Agent is a serializable representation of a dive.Agent
type Agent struct {
	Name               string   `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description        string   `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Instructions       string   `yaml:"instructions,omitempty" json:"instructions,omitempty" hcl:"instructions,optional"`
	IsSupervisor       bool     `yaml:"is_supervisor,omitempty" json:"is_supervisor,omitempty" hcl:"is_supervisor,optional"`
	Subordinates       []string `yaml:"subordinates,omitempty" json:"subordinates,omitempty" hcl:"subordinates,optional"`
	AcceptedEvents     []string `yaml:"accepted_events,omitempty" json:"accepted_events,omitempty" hcl:"accepted_events,optional"`
	Provider           string   `yaml:"provider,omitempty" json:"provider,omitempty" hcl:"provider,optional"`
	Model              string   `yaml:"model,omitempty" json:"model,omitempty" hcl:"model,optional"`
	Tools              []string `yaml:"tools,omitempty" json:"tools,omitempty" hcl:"tools,optional"`
	CacheControl       string   `yaml:"cache_control,omitempty" json:"cache_control,omitempty" hcl:"cache_control,optional"`
	TaskTimeout        string   `yaml:"task_timeout,omitempty" json:"task_timeout,omitempty" hcl:"task_timeout,optional"`
	ChatTimeout        string   `yaml:"chat_timeout,omitempty" json:"chat_timeout,omitempty" hcl:"chat_timeout,optional"`
	ToolIterationLimit int      `yaml:"tool_iteration_limit,omitempty" json:"tool_iteration_limit,omitempty" hcl:"tool_iteration_limit,optional"`
	LogLevel           string   `yaml:"log_level,omitempty" json:"log_level,omitempty" hcl:"log_level,optional"`
}

// Task is a serializable representation of a dive.Task
type Task struct {
	Name           string   `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description    string   `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	ExpectedOutput string   `yaml:"expected_output,omitempty" json:"expected_output,omitempty" hcl:"expected_output,optional"`
	OutputFormat   string   `yaml:"output_format,omitempty" json:"output_format,omitempty" hcl:"output_format,optional"`
	AssignedAgent  string   `yaml:"assigned_agent,omitempty" json:"assigned_agent,omitempty" hcl:"assigned_agent,optional"`
	Dependencies   []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty" hcl:"dependencies,optional"`
	Documents      []string `yaml:"documents,omitempty" json:"documents,omitempty" hcl:"documents,optional"`
	OutputFile     string   `yaml:"output_file,omitempty" json:"output_file,omitempty" hcl:"output_file,optional"`
	Timeout        string   `yaml:"timeout,omitempty" json:"timeout,omitempty" hcl:"timeout,optional"`
	Context        string   `yaml:"context,omitempty" json:"context,omitempty" hcl:"context,optional"`
}

// ToolDefinition used for serializing tool configurations
type ToolDefinition map[string]interface{}

// Document represents a document that can be referenced by agents and tasks
type Document struct {
	ID          string   `yaml:"id,omitempty" json:"id,omitempty" hcl:"id,label"`
	Name        string   `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Path        string   `yaml:"path,omitempty" json:"path,omitempty" hcl:"path,optional"`
	URI         string   `yaml:"uri,omitempty" json:"uri,omitempty" hcl:"uri,optional"`
	Content     string   `yaml:"content,omitempty" json:"content,omitempty" hcl:"content,optional"`
	ContentType string   `yaml:"content_type,omitempty" json:"content_type,omitempty" hcl:"content_type,optional"`
	References  []string `yaml:"references,omitempty" json:"references,omitempty" hcl:"references,optional"`
}

// LoadFile loads a Team configuration from a file. The file extension is
// used to determine the configuration format:
// - .json -> JSON
// - .yml or .yaml -> YAML
// - .hcl or .dive -> HCL
func LoadFile(filePath string, variables map[string]interface{}) (*Team, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	filename := filepath.Base(filePath)
	extension := filepath.Ext(filePath)

	switch extension {
	case ".json":
		return LoadJSON(data)
	case ".yml", ".yaml":
		return LoadYAML(data)
	case ".hcl", ".dive":
		return LoadHCL(data, filename, variables)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", extension)
	}
}

// LoadJSON loads a Team configuration from a JSON string
func LoadJSON(conf []byte) (*Team, error) {
	var def Team
	if err := json.Unmarshal([]byte(conf), &def); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return &def, nil
}

// LoadYAML loads a Team configuration from a YAML string
func LoadYAML(conf []byte) (*Team, error) {
	var def Team
	if err := yaml.Unmarshal([]byte(conf), &def); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	return &def, nil
}

// LoadHCL loads a Team configuration from a HCL string
func LoadHCL(conf []byte, filename string, variables map[string]interface{}) (*Team, error) {
	hclteam, err := LoadHCLDefinition(conf, filename, variables)
	if err != nil {
		return nil, err
	}

	var teamVariables []Variable
	for _, v := range hclteam.Variables {
		teamVariables = append(teamVariables, Variable{
			Name:        v.Name,
			Type:        v.Type,
			Description: v.Description,
			// Default:     v.Default.,
		})
	}

	var tools []ToolDefinition
	for _, t := range hclteam.Tools {
		tools = append(tools, map[string]interface{}{
			"name":    t.Name,
			"enabled": t.Enabled,
			// TODO: ... parameters
		})
	}

	// Convert HCLTeam to Team
	def := &Team{
		Name:        hclteam.Name,
		Description: hclteam.Description,
		Agents:      hclteam.Agents,
		Tasks:       hclteam.Tasks,
		Config:      hclteam.Config,
		Variables:   teamVariables,
		Tools:       tools,
	}
	return def, nil
}

// TeamFromFile loads a team configuration from a file and builds it, returning
// the usable dive.Team. This is a convenience function that combines the load
// and build steps.
func TeamFromFile(filePath string, opts ...BuildOption) (dive.Team, error) {
	buildOpts := &buildOptions{}
	for _, opt := range opts {
		opt(buildOpts)
	}
	conf, err := LoadFile(filePath, buildOpts.Variables)
	if err != nil {
		return nil, err
	}
	return conf.Build(opts...)
}

type buildOptions struct {
	Variables     map[string]interface{}
	Logger        slogger.Logger
	DocumentStore dive.DocumentStore
}

type BuildOption func(*buildOptions)

func WithVariables(vars map[string]interface{}) BuildOption {
	return func(o *buildOptions) {
		o.Variables = vars
	}
}

func WithLogger(logger slogger.Logger) BuildOption {
	return func(o *buildOptions) {
		o.Logger = logger
	}
}

func WithDocumentStore(store dive.DocumentStore) BuildOption {
	return func(o *buildOptions) {
		o.DocumentStore = store
	}
}

func (def *Team) Build(opts ...BuildOption) (dive.Team, error) {
	buildOpts := &buildOptions{}
	for _, opt := range opts {
		opt(buildOpts)
	}

	logLevel := "info"
	if def.Config.LogLevel != "" {
		logLevel = def.Config.LogLevel
	}

	logger := buildOpts.Logger
	if logger == nil {
		logger = slogger.New(slogger.LevelFromString(logLevel))
	}

	var toolConfigs map[string]map[string]interface{}
	if def.Tools != nil {
		toolConfigs = make(map[string]map[string]interface{}, len(def.Tools))
		for _, toolDef := range def.Tools {
			name, ok := toolDef["name"].(string)
			if !ok {
				return nil, fmt.Errorf("tool name is missing")
			}
			toolConfigs[name] = toolDef
		}
	}

	toolsMap, err := initializeTools(toolConfigs)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tools: %w", err)
	}

	agents := make([]dive.Agent, 0, len(def.Agents))
	for _, agentDef := range def.Agents {
		agent, err := buildAgent(agentDef, def.Config, toolsMap, logger, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", agentDef.Name, err)
		}
		agents = append(agents, agent)
	}

	tasks := make([]*dive.Task, 0, len(def.Tasks))
	for _, taskDef := range def.Tasks {
		task, err := buildTask(taskDef, agents, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build task %s: %w", taskDef.Name, err)
		}
		tasks = append(tasks, task)
	}

	if len(def.Documents) > 0 && buildOpts.DocumentStore == nil {
		return nil, fmt.Errorf("document store is required when documents are defined")
	}

	documents := make(map[string]dive.Document, len(def.Documents))
	for _, docRef := range def.Documents {
		doc, err := ResolveDocument(context.Background(), buildOpts.DocumentStore, docRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve document %s: %w", docRef.Name, err)
		}
		documents[docRef.Name] = doc
	}

	return dive.NewTeam(dive.TeamOptions{
		Name:        def.Name,
		Description: def.Description,
		Agents:      agents,
		Tasks:       tasks,
		Documents:   buildOpts.DocumentStore,
		Logger:      logger,
		LogLevel:    logLevel,
		OutputDir:   def.Config.OutputDir,
	})
}

func ResolveDocument(ctx context.Context, store dive.DocumentStore, ref Document) (dive.Document, error) {
	// If content is provided directly, create an in-memory document
	if ref.Content != "" {
		return dive.NewTextDocument(dive.DocumentOptions{
			ID:          ref.ID,
			Name:        ref.Name,
			Description: ref.Description,
			Content:     ref.Content,
			ContentType: ref.ContentType,
			Tags:        ref.References, // Use References as tags
		}), nil
	}

	// If path is provided, create a document with that URI
	if ref.Path != "" {
		// If path is a glob pattern, it will be handled by the document store
		// when listing documents
		doc := dive.NewTextDocument(dive.DocumentOptions{
			ID:          ref.ID,
			Name:        ref.Name,
			Description: ref.Description,
			URI:         ref.Path,
			ContentType: ref.ContentType,
			Tags:        ref.References,
		})

		// Store the document so it can be retrieved later
		if err := store.PutDocument(ctx, doc); err != nil {
			return nil, fmt.Errorf("failed to store document %q: %w", ref.Name, err)
		}

		return doc, nil
	}

	// If URI is provided, create a document with that URI
	if ref.URI != "" {
		doc := dive.NewTextDocument(dive.DocumentOptions{
			ID:          ref.ID,
			Name:        ref.Name,
			Description: ref.Description,
			URI:         ref.URI,
			ContentType: ref.ContentType,
			Tags:        ref.References,
		})

		// Store the document so it can be retrieved later
		if err := store.PutDocument(ctx, doc); err != nil {
			return nil, fmt.Errorf("failed to store document %q: %w", ref.Name, err)
		}

		return doc, nil
	}

	return nil, fmt.Errorf("document %q must have either content, path, or uri", ref.Name)
}
