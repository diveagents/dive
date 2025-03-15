package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getstingrai/dive/environment"
	"gopkg.in/yaml.v3"
)

// ParseFile loads an Environment configuration from a file. The file extension
// is used to determine the configuration format (JSON or YAML).
func ParseFile(path string) (*Environment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return ParseJSON(data)
	case ".yml", ".yaml":
		return ParseYAML(data)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// ParseYAML loads an Environment configuration from YAML
func ParseYAML(data []byte) (*Environment, error) {
	var env Environment
	if err := yaml.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// ParseJSON loads an Environment configuration from JSON
func ParseJSON(data []byte) (*Environment, error) {
	var env Environment
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// LoadDirectory loads all YAML and JSON files from a directory and combines
// them into a single Environment. Files are loaded in lexicographical order.
// Later files can override values from earlier files.
func LoadDirectory(dirPath string, opts ...BuildOption) (*environment.Environment, error) {
	// Read all files in the directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// Collect all YAML and JSON files
	var configFiles []string
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".yml" || ext == ".yaml" || ext == ".json" {
				configFiles = append(configFiles, filepath.Join(dirPath, entry.Name()))
			}
		}
	}

	// Sort files for deterministic loading order
	sort.Strings(configFiles)

	// Consider an empty directory an error
	if len(configFiles) == 0 {
		return nil, fmt.Errorf("no yaml or json files found in directory: %s", dirPath)
	}

	// Merge all configuration files
	var merged *Environment
	for _, file := range configFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", file, err)
		}

		var env *Environment
		ext := strings.ToLower(filepath.Ext(file))
		if ext == ".json" {
			env, err = ParseJSON(data)
		} else {
			env, err = ParseYAML(data)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to parse file %s: %w", file, err)
		}

		if merged == nil {
			merged = env
		} else {
			merged = Merge(merged, env)
		}
	}

	return merged.Build(opts...)
}
