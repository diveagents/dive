package document

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
	Path() string
	Version() int
	Content() string
	ContentType() string
	Tags() []string
	Chunks() []*Chunk
}
