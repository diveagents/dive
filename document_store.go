package dive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemDocumentStore implements DocumentStore using the local file system
type FileSystemDocumentStore struct {
	rootDir string
}

// NewFileSystemDocumentStore creates a new document store backed by the file system
func NewFileSystemDocumentStore(rootDir string) (*FileSystemDocumentStore, error) {
	if rootDir == "" {
		rootDir = "."
	}
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}
	return &FileSystemDocumentStore{
		rootDir: rootDir,
	}, nil
}

// GetDocument returns a document by name (which is treated as a path)
func (s *FileSystemDocumentStore) GetDocument(ctx context.Context, name string) (Document, error) {
	path := filepath.Join(s.rootDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("document %q does not exist", name)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read document %q: %w", name, err)
	}
	return NewTextDocument(DocumentOptions{
		Name:        filepath.Base(name),
		Path:        name,
		Content:     string(content),
		ContentType: detectContentType(name),
	}), nil
}

// ListDocuments lists documents matching the input criteria
func (s *FileSystemDocumentStore) ListDocuments(ctx context.Context, input *ListDocumentInput) (*ListDocumentOutput, error) {
	var docs []Document
	startDir := filepath.Join(s.rootDir, input.PathPrefix)

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(s.rootDir, path)
		if err != nil {
			return err
		}
		if input.PathPrefix != "" && !strings.HasPrefix(relPath, input.PathPrefix) {
			return nil
		}
		// Skip directories unless this is the start dir
		if info.IsDir() {
			// If not recursive and this isn't the start dir, skip this directory
			if !input.Recursive && path != startDir {
				return filepath.SkipDir
			}
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}
		doc := NewTextDocument(DocumentOptions{
			Name:        filepath.Base(relPath),
			Path:        relPath,
			Content:     string(content),
			ContentType: detectContentType(relPath),
		})
		// Filter by tags if specified
		if len(input.Tags) > 0 && !hasAllTags(doc.Tags(), input.Tags) {
			return nil
		}
		docs = append(docs, doc)
		return nil
	}

	err := filepath.Walk(startDir, walkFn)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to walk directory %q: %w", input.PathPrefix, err)
	}
	return &ListDocumentOutput{Items: docs}, nil
}

// PutDocument puts a document into the store
func (s *FileSystemDocumentStore) PutDocument(ctx context.Context, doc Document) error {
	if doc.Path() == "" {
		return fmt.Errorf("document path is required")
	}
	path := filepath.Join(s.rootDir, doc.Path())
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory for document: %w", err)
	}
	if err := os.WriteFile(path, []byte(doc.Content()), 0644); err != nil {
		return fmt.Errorf("failed to write document to file: %w", err)
	}
	return nil
}

// DeleteDocument deletes a document from the store
func (s *FileSystemDocumentStore) DeleteDocument(ctx context.Context, doc Document) error {
	if doc.Path() == "" {
		return fmt.Errorf("document path is required")
	}
	path := filepath.Join(s.rootDir, doc.Path())
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete document file: %w", err)
	}
	return nil
}

// Helper functions

func detectContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".html", ".htm":
		return "text/html"
	default:
		return "text/plain"
	}
}

func hasAllTags(docTags, searchTags []string) bool {
	if len(searchTags) == 0 {
		return true
	}
	if len(docTags) == 0 {
		return false
	}

	tagSet := make(map[string]bool)
	for _, tag := range docTags {
		tagSet[tag] = true
	}

	for _, tag := range searchTags {
		if !tagSet[tag] {
			return false
		}
	}
	return true
}
