package environment

import (
	"context"
	"encoding/json"
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/diveagents/dive/llm"
	"github.com/risor-io/risor"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/modules/all"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
)

// DynamicContent is an interface that allows content to be dynamically
// generated at runtime during workflow execution.
type DynamicContent interface {
	Content(ctx context.Context, inputs map[string]any) ([]llm.Content, error)
}

//// RisorContent //////////////////////////////////////////////////////////////

// RisorContent represents content that is dynamically generated by executing
// a Risor script at runtime.
type RisorContent struct {
	Dynamic  string `json:"dynamic"`
	BasePath string `json:"base_path,omitempty"`
}

func (c *RisorContent) Type() llm.ContentType {
	return llm.ContentTypeDynamic
}

func (c *RisorContent) Content(ctx context.Context, globals map[string]any) ([]llm.Content, error) {
	if globals == nil {
		globals = make(map[string]any)
	}

	// Execute the Risor script
	result, err := c.executeRisorScript(ctx, globals)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Risor script: %w", err)
	}

	// Convert the result to content blocks
	return c.convertScriptResultToContent(result)
}

// executeRisorScript compiles and executes a Risor script
func (c *RisorContent) executeRisorScript(ctx context.Context, globals map[string]any) (interface{}, error) {
	ast, err := parser.Parse(ctx, c.Dynamic)
	if err != nil {
		return nil, fmt.Errorf("failed to parse script: %w", err)
	}

	globalsCopy := make(map[string]any)
	for k, v := range globals {
		globalsCopy[k] = v
	}

	// Make Risor's default builtins available to embedded scripts
	for k, v := range all.Builtins() {
		globalsCopy[k] = v
	}

	var globalNames []string
	for name := range globalsCopy {
		globalNames = append(globalNames, name)
	}
	sort.Strings(globalNames)

	compiledCode, err := compiler.Compile(ast, compiler.WithGlobalNames(globalNames))
	if err != nil {
		return nil, fmt.Errorf("failed to compile script: %w", err)
	}

	result, err := risor.EvalCode(ctx, compiledCode, risor.WithGlobals(globalsCopy))
	if err != nil {
		return nil, fmt.Errorf("failed to execute script: %w", err)
	}

	// Convert Risor object to Go interface
	return c.convertRisorObjectToInterface(result), nil
}

// convertRisorObjectToInterface converts a Risor object to a Go interface
func (c *RisorContent) convertRisorObjectToInterface(obj object.Object) interface{} {
	switch o := obj.(type) {
	case *object.String:
		return o.Value()
	case *object.Int:
		return o.Value()
	case *object.Float:
		return o.Value()
	case *object.Bool:
		return o.Value()
	case *object.List:
		var result []interface{}
		for _, item := range o.Value() {
			result = append(result, c.convertRisorObjectToInterface(item))
		}
		return result
	case *object.Map:
		result := make(map[string]interface{})
		for key, value := range o.Value() {
			result[key] = c.convertRisorObjectToInterface(value)
		}
		return result
	default:
		// Fallback to string representation
		return obj.Inspect()
	}
}

// convertScriptResultToContent converts script result to Content blocks
func (c *RisorContent) convertScriptResultToContent(result interface{}) ([]llm.Content, error) {
	switch v := result.(type) {
	case string:
		// Simple text content
		return []llm.Content{&llm.TextContent{Text: v}}, nil
	case map[string]interface{}:
		// Single content block
		data, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal content block: %w", err)
		}
		content, err := llm.UnmarshalContent(data)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal content block: %w", err)
		}
		return []llm.Content{content}, nil
	case []interface{}:
		// Multiple content blocks
		var contents []llm.Content
		for _, item := range v {
			data, err := json.Marshal(item)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal content block: %w", err)
			}
			content, err := llm.UnmarshalContent(data)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal content block: %w", err)
			}
			contents = append(contents, content)
		}
		return contents, nil
	default:
		return nil, fmt.Errorf("unsupported script result type: %T", result)
	}
}

// convertMapToContent converts a map to a Content object
func (c *RisorContent) convertMapToContent(m map[string]interface{}) (llm.Content, error) {
	contentType, ok := m["type"].(string)
	if !ok {
		return nil, fmt.Errorf("content block missing 'type' field")
	}

	switch contentType {
	case "text":
		text, ok := m["text"].(string)
		if !ok {
			return nil, fmt.Errorf("text content block missing 'text' field")
		}
		return &llm.TextContent{Text: text}, nil
	case "image":
		source, err := c.convertMapToContentSource(m)
		if err != nil {
			return nil, fmt.Errorf("failed to convert image source: %w", err)
		}
		return &llm.ImageContent{Source: source}, nil
	case "document":
		source, err := c.convertMapToContentSource(m)
		if err != nil {
			return nil, fmt.Errorf("failed to convert document source: %w", err)
		}
		return &llm.DocumentContent{Source: source}, nil
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// convertMapToContentSource converts a map to a ContentSource
func (c *RisorContent) convertMapToContentSource(m map[string]interface{}) (*llm.ContentSource, error) {
	source := &llm.ContentSource{}

	if sourceType, ok := m["source_type"].(string); ok {
		switch sourceType {
		case "url":
			source.Type = llm.ContentSourceTypeURL
		case "base64":
			source.Type = llm.ContentSourceTypeBase64
		default:
			return nil, fmt.Errorf("unsupported source type: %s", sourceType)
		}
	}

	if mediaType, ok := m["media_type"].(string); ok {
		source.MediaType = mediaType
	}

	if url, ok := m["url"].(string); ok {
		source.URL = url
	}

	if data, ok := m["data"].(string); ok {
		source.Data = data
	}

	return source, nil
}

//// ScriptPathContent /////////////////////////////////////////////////////////

// ScriptPathContent represents content that is dynamically generated by executing
// an external script at runtime.
type ScriptPathContent struct {
	DynamicFrom string `json:"dynamic_from"`
	BasePath    string `json:"base_path,omitempty"`
}

func (c *ScriptPathContent) Type() llm.ContentType {
	return llm.ContentTypeDynamic
}

func (c *ScriptPathContent) Content(ctx context.Context, globals map[string]any) ([]llm.Content, error) {
	if globals == nil {
		globals = make(map[string]any)
	}
	// Resolve the script path
	resolvedPath := c.DynamicFrom
	if !filepath.IsAbs(c.DynamicFrom) && c.BasePath != "" {
		resolvedPath = filepath.Join(c.BasePath, c.DynamicFrom)
	}
	// Check if file exists
	if _, err := os.Stat(resolvedPath); err != nil {
		return nil, fmt.Errorf("script file not found: %s", resolvedPath)
	}
	// Execute as external script and expect JSON output
	return c.executeExternalScript(ctx, resolvedPath, globals)
}

// executeExternalScript executes an external script and parses JSON output
func (c *ScriptPathContent) executeExternalScript(ctx context.Context, scriptPath string, globals map[string]any) ([]llm.Content, error) {
	cmd := exec.CommandContext(ctx, scriptPath)

	env := os.Environ()
	// TODO: insert variables?
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w", err)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(output, &items); err != nil {
		return nil, fmt.Errorf("failed to parse script JSON output: %w", err)
	}
	var resultItems []llm.Content
	for _, item := range items {
		resultItem, err := llm.UnmarshalContent(item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse script JSON output: %w", err)
		}
		resultItems = append(resultItems, resultItem)
	}
	return resultItems, nil
}
