package dive

import "context"

var (
	_ Document = &TextDocument{}
)

// DocumentRef is used to point to one or more matching documents
type DocumentRef struct {
	Name string   `json:"name,omitempty"`
	Tags []string `json:"tags,omitempty"`
	Glob string   `json:"glob,omitempty"`
}

// Chunk of a document. I'm not yet sure if this should have a pointer to a
// Document or if it should be referenced by name/id/path. :thinking:
type Chunk struct {
	Index      int      `json:"index"`
	Content    string   `json:"content"`
	Heading    string   `json:"heading,omitempty"`
	DocumentID string   `json:"document_id,omitempty"`
	Document   Document `json:"-"`
}

// Document containing content that can be read or written to by an agent
type Document interface {
	ID() string
	Name() string
	Description() string
	URI() string
	Version() int
	Content() string
	ContentType() string
	Tags() []string
	Chunks() []*Chunk
}

// ListDocumentInput specifies search criteria for documents in a document store
type ListDocumentInput struct {
	PathPrefix string
	Recursive  bool
	Tags       []string
}

// ListDocumentOutput is the output for listing documents
type ListDocumentOutput struct {
	Items []Document
}

// DocumentStore provides read/write access to a set of documents. This could be
// backed by a local file system, a remote API, or a database.
type DocumentStore interface {
	// GetDocument returns a document by name
	GetDocument(ctx context.Context, name string) (Document, error)

	// ListDocuments lists documents
	ListDocuments(ctx context.Context, input *ListDocumentInput) (*ListDocumentOutput, error)

	// PutDocument puts a document
	PutDocument(ctx context.Context, doc Document) error

	// DeleteDocument deletes a document
	DeleteDocument(ctx context.Context, doc Document) error
}

type DocumentOptions struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	URI         string   `json:"uri,omitempty"`
	Version     int      `json:"version,omitempty"`
	Content     string   `json:"content,omitempty"`
	ContentType string   `json:"content_type,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func NewTextDocument(opts DocumentOptions) *TextDocument {
	if opts.ContentType == "" {
		opts.ContentType = "text/plain"
	}
	return &TextDocument{
		id:          opts.ID,
		name:        opts.Name,
		description: opts.Description,
		uri:         opts.URI,
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
	uri         string
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

func (d *TextDocument) URI() string {
	return d.uri
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

func (d *TextDocument) SetURI(uri string) {
	d.uri = uri
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
