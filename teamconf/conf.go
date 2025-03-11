package teamconf

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/document"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
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
}

// AgentConfig is a serializable representation of a dive.Agent
type AgentConfig struct {
	Name         string         `yaml:"name,omitempty" json:"name,omitempty" hcl:"name,label"`
	Description  string         `yaml:"description,omitempty" json:"description,omitempty" hcl:"description,optional"`
	Instructions string         `yaml:"instructions,omitempty" json:"instructions,omitempty" hcl:"instructions,optional"`
	Tools        []string       `yaml:"tools,omitempty" json:"tools,omitempty" hcl:"tools,optional"`
	IsSupervisor bool           `yaml:"is_supervisor,omitempty" json:"is_supervisor,omitempty" hcl:"is_supervisor,optional"`
	Subordinates []string       `yaml:"subordinates,omitempty" json:"subordinates,omitempty" hcl:"subordinates,optional"`
	TaskTimeout  string         `yaml:"task_timeout,omitempty" json:"task_timeout,omitempty" hcl:"task_timeout,optional"`
	ChatTimeout  string         `yaml:"chat_timeout,omitempty" json:"chat_timeout,omitempty" hcl:"chat_timeout,optional"`
	LogLevel     string         `yaml:"log_level,omitempty" json:"log_level,omitempty" hcl:"log_level,optional"`
	ToolConfig   map[string]any `yaml:"tool_config,omitempty" json:"tool_config,omitempty" hcl:"tool_config,block"`
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
	Content     string   `yaml:"content,omitempty" json:"content,omitempty" hcl:"content,optional"`
	ContentType string   `yaml:"content_type,omitempty" json:"content_type,omitempty" hcl:"content_type,optional"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty" hcl:"tags,optional"`
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
	return loadHCL(data)
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

// Build creates a new Team from the configuration
func (def *TeamDefinition) Build(buildOpts BuildOptions) (dive.Team, error) {
	// Create agents
	agents := make([]dive.Agent, 0, len(def.Agents))
	for _, agentDef := range def.Agents {
		agent, err := buildAgent(agentDef, buildOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to build agent %s: %w", agentDef.Name, err)
		}
		agents = append(agents, agent)
	}

	// Create document store
	docStore := document.NewMemoryStore()
	for _, docDef := range def.Documents {
		doc := document.NewTextDocument(document.DocumentOptions{
			ID:          docDef.ID,
			Name:        docDef.Name,
			Description: docDef.Description,
			Path:        docDef.Path,
			Content:     docDef.Content,
			ContentType: docDef.ContentType,
			Tags:        docDef.Tags,
		})
		if err := docStore.AddDocument(doc); err != nil {
			return nil, fmt.Errorf("failed to add document %s: %w", docDef.Name, err)
		}
	}

	// Create tasks
	tasks := make([]*workflow.Task, 0, len(def.Tasks))
	for _, taskDef := range def.Tasks {
		task, err := buildTask(taskDef, agents, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build task %s: %w", taskDef.Name, err)
		}
		tasks = append(tasks, task)
	}

	// Create team
	team := dive.NewTeam(dive.TeamOptions{
		Name:        def.Name,
		Description: def.Description,
		Agents:      agents,
		Tasks:       tasks,
		Documents:   docStore,
		LogLevel:    buildOpts.LogLevel,
		Logger:      buildOpts.Logger,
		OutputDir:   buildOpts.OutputDir,
	})

	return team, nil
}

// Write writes a Team configuration to a writer in YAML format
func (def *TeamDefinition) Write(w io.Writer) error {
	return yaml.NewEncoder(w).Encode(def)
}

func (def *TeamDefinition) Build(opts ...BuildOption) (dive.Team, error) {
	buildOpts := &BuildOptions{}
	for _, opt := range opts {
		opt(buildOpts)
	}

	logLevel := "info"
	if def.Config != nil {
		if logLevelVal, ok := def.Config["log_level"]; ok {
			logLevel = logLevelVal.(string)
		}
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

	steps := make([]*workflow.Task, 0, len(def.Tasks))
	for _, taskDef := range def.Tasks {
		task, err := buildTask(taskDef, agents, buildOpts.Variables)
		if err != nil {
			return nil, fmt.Errorf("failed to build task %s: %w", taskDef.Name, err)
		}
		steps = append(steps, task)
	}

	if len(def.Documents) > 0 && buildOpts.Repository == nil {
		return nil, fmt.Errorf("repository is required when documents are defined")
	}

	documents := make(map[string]document.Document, len(def.Documents))
	for _, docRef := range def.Documents {
		doc, err := ResolveDocument(context.Background(), buildOpts.Repository, docRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve document %s: %w", docRef.Name, err)
		}
		documents[docRef.Name] = doc
	}

	return workflow.NewTeam(workflow.TeamOptions{
		Name:        def.Name,
		Description: def.Description,
		Agents:      agents,
		Tasks:       steps,
		Documents:   buildOpts.Repository,
		Logger:      logger,
		LogLevel:    logLevel,
		OutputDir:   def.Config["output_dir"].(string),
	})
}

func ResolveDocument(ctx context.Context, store document.DocumentStore, ref Document) (document.Document, error) {
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
