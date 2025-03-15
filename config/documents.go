package config

import (
	"context"
	"fmt"

	"github.com/getstingrai/dive/document"
)

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
