package document

import "path/filepath"

type DocumentOptions struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Path        string   `json:"path,omitempty"`
	Version     int      `json:"version,omitempty"`
	Content     string   `json:"content,omitempty"`
	ContentType string   `json:"content_type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func NewTextDocument(opts DocumentOptions) *TextDocument {
	if opts.ContentType == "" {
		opts.ContentType = "text/plain"
	}

	// Validate name matches path basename if both are provided
	if opts.Name != "" && opts.Path != "" {
		basename := filepath.Base(opts.Path)
		if basename != opts.Name {
			// Auto-correct to match the path. Could panic instead...
			opts.Name = basename
		}
	} else if opts.Path != "" {
		// If only path is provided, derive name from it
		opts.Name = filepath.Base(opts.Path)
	}

	return &TextDocument{
		id:          opts.ID,
		name:        opts.Name,
		description: opts.Description,
		path:        opts.Path,
		version:     opts.Version,
		content:     opts.Content,
		contentType: opts.ContentType,
		tags:        opts.Tags,
	}
}

type TextDocument struct {
	id          string
	name        string
	description string
	path        string
	version     int
	content     string
	contentType string
	tags        []string
}

func (d *TextDocument) ID() string {
	return d.id
}

func (d *TextDocument) Name() string {
	return d.name
}

func (d *TextDocument) Path() string {
	return d.path
}

func (d *TextDocument) Version() int {
	return d.version
}

func (d *TextDocument) Description() string {
	return d.description
}

func (d *TextDocument) Content() string {
	return d.content
}

func (d *TextDocument) ContentType() string {
	return d.contentType
}

func (d *TextDocument) Tags() []string {
	return d.tags
}

func (d *TextDocument) SetName(name string) {
	d.name = name
}

func (d *TextDocument) SetPath(path string) {
	d.path = path
}

func (d *TextDocument) SetContent(content string) {
	d.content = content
}

func (d *TextDocument) IncrementVersion() {
	d.version++
}

func (d *TextDocument) Chunks() []*Chunk {
	// TODO
	return []*Chunk{}
}
