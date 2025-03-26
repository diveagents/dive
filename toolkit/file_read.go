package toolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/diveagents/dive/llm"
)

const DefaultFileReadMaxSize = 1024 * 200 // 200KB

type FileReadInput struct {
	Path string `json:"path"`
}

type FileReadToolOptions struct {
	DefaultFilePath string `json:"default_file_path,omitempty"`
	MaxSize         int    `json:"max_size,omitempty"`
	RootDirectory   string `json:"root_directory,omitempty"`
}

type FileReadTool struct {
	defaultFilePath string
	maxSize         int
	rootDirectory   string
}

// NewFileReadTool creates a new tool for reading file contents
func NewFileReadTool(options FileReadToolOptions) *FileReadTool {
	if options.MaxSize <= 0 {
		options.MaxSize = DefaultFileReadMaxSize
	}
	return &FileReadTool{
		defaultFilePath: options.DefaultFilePath,
		maxSize:         options.MaxSize,
		rootDirectory:   options.RootDirectory,
	}
}

func (t *FileReadTool) Definition() *llm.ToolDefinition {
	description := "A tool that reads the content of a file. To use this tool, provide a 'path' parameter with the path to the file you want to read."

	if t.defaultFilePath != "" {
		description = fmt.Sprintf("A tool that reads file content. The default file is %s, but you can provide a different 'path' parameter to read another file.", t.defaultFilePath)
	}

	if t.rootDirectory != "" {
		description += fmt.Sprintf(" All paths are relative to %s.", t.rootDirectory)
	}

	return &llm.ToolDefinition{
		Name:        "file_read",
		Description: description,
		Parameters: llm.Schema{
			Type:     "object",
			Required: []string{"path"},
			Properties: map[string]*llm.SchemaProperty{
				"path": {
					Type:        "string",
					Description: "Path to the file to be read",
				},
			},
		},
	}
}

// resolvePath resolves the provided path, applying rootDirectory if configured
// and preventing directory traversal attacks
func (t *FileReadTool) resolvePath(path string) (string, error) {
	if t.rootDirectory == "" {
		// If no root directory is set, use the path as is
		return path, nil
	}

	// Join the root directory and the provided path
	resolvedPath := filepath.Join(t.rootDirectory, path)

	// Get the absolute paths to check for directory traversal
	absRoot, err := filepath.Abs(t.rootDirectory)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for root directory: %w", err)
	}

	absPath, err := filepath.Abs(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Check if the resolved path is within the root directory
	if !filepath.HasPrefix(absPath, absRoot) {
		return "", fmt.Errorf("path attempts to access location outside of root directory")
	}

	return resolvedPath, nil
}

func (t *FileReadTool) Call(ctx context.Context, input string) (string, error) {
	var params FileReadInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", err
	}

	filePath := params.Path
	if filePath == "" {
		filePath = t.defaultFilePath
	}

	if filePath == "" {
		return "Error: No file path provided. Please provide a file path either in the constructor or as an argument.", nil
	}

	// Resolve the path (apply rootDirectory if configured)
	resolvedPath, err := t.resolvePath(filePath)
	if err != nil {
		return fmt.Sprintf("Error: %s", err.Error()), nil
	}

	// Get file info to check size before reading
	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Sprintf("Error: File not found at path: %s", filePath), nil
		} else if os.IsPermission(err) {
			return fmt.Sprintf("Error: Permission denied when trying to access file: %s", filePath), nil
		}
		return fmt.Sprintf("Error: Failed to access file %s. %s", filePath, err.Error()), nil
	}

	// Check if file size exceeds the maximum size before reading it
	if fileInfo.Size() > int64(t.maxSize) {
		return fmt.Sprintf("Error: File %s is too large (%d bytes). Maximum allowed size is %d bytes.", filePath, fileInfo.Size(), t.maxSize), nil
	}

	// Now that we know the file is not too large, read its content
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return fmt.Sprintf("Error: Failed to read file %s. %s", filePath, err.Error()), nil
	}

	// Check if the file appears to be binary
	if isBinaryContent(content) {
		return fmt.Sprintf("Warning: File %s appears to be a binary file. The content may not display correctly:\n\n%s",
			filePath, string(content)), nil
	}

	return string(content), nil
}

// isBinaryContent attempts to determine if the content is binary by checking for null bytes
// and examining the ratio of control characters to printable characters
func isBinaryContent(content []byte) bool {
	// Quick check: if there are null bytes, it's likely binary
	if bytes.Contains(content, []byte{0}) {
		return true
	}

	// Check a sample of the file (up to first 512 bytes)
	sampleSize := 512
	if len(content) < sampleSize {
		sampleSize = len(content)
	}

	sample := content[:sampleSize]
	controlCount := 0

	for _, b := range sample {
		// Count control characters (except common whitespace)
		if (b < 32 && b != 9 && b != 10 && b != 13) || b > 126 {
			controlCount++
		}
	}

	// If more than 10% are control characters, likely binary
	return controlCount > sampleSize/10
}

func (t *FileReadTool) ShouldReturnResult() bool {
	return true
}
