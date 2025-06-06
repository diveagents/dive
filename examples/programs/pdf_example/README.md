# PDF Support Example

This example demonstrates how to use PDF support with both Anthropic and OpenAI Responses API providers in the Dive framework using the unified `DocumentContent` approach.

## Features

- **Unified PDF Support**: Uses `DocumentContent` with base64, URL, or Files API references across all providers
- **Multiple Input Methods**: Support for local files and remote URLs
- **Cross-Provider Compatibility**: Same interface works with both providers
- **Anthropic API Alignment**: Follows Anthropic's PDF support specification

## Usage

### Analyze a PDF from URL

```bash
# Using Anthropic (default)
go run main.go -url "https://example.com/document.pdf" -prompt "Summarize this document"

# Using OpenAI Responses API
go run main.go -provider openai-responses -url "https://example.com/document.pdf" -prompt "What are the key points?"
```

### Analyze a Local PDF File

```bash
# Using Anthropic
go run main.go -pdf "./document.pdf" -prompt "Extract the main findings"

# Using OpenAI Responses API
go run main.go -provider openai-responses -pdf "./document.pdf" -prompt "List the conclusions"
```

### Command Line Options

- `-provider`: LLM provider to use (`anthropic` or `openai-responses`)
- `-pdf`: Path to local PDF file
- `-url`: URL to remote PDF file
- `-prompt`: Analysis prompt (default: "What are the key findings in this document?")
- `-log`: Log level (`debug`, `info`, `warn`, `error`)

## Implementation Details

### Unified DocumentContent Approach

Both providers now use the same `DocumentContent` structure, following Anthropic's API specification:

```go
// Base64 encoded PDF
&llm.DocumentContent{
    Title: "document.pdf",
    Source: &llm.ContentSource{
        Type:      llm.ContentSourceTypeBase64,
        MediaType: "application/pdf",
        Data:      "JVBERi0x...", // base64 data without data URI prefix
    },
}

// URL reference
&llm.DocumentContent{
    Title: "Remote Document",
    Source: &llm.ContentSource{
        Type: llm.ContentSourceTypeURL,
        URL:  "https://example.com/document.pdf",
    },
}

// Files API reference
&llm.DocumentContent{
    Title: "API Document",
    Source: &llm.ContentSource{
        Type:   llm.ContentSourceTypeFile,
        FileID: "file-abc123",
    },
}
```

### Helper Functions

The framework provides several helper functions for creating PDF messages:

```go
// Preferred unified approach
llm.NewUserDocumentMessage("title", "application/pdf", base64Data)
llm.NewUserDocumentURLMessage("title", "https://example.com/doc.pdf")
llm.NewUserDocumentFileIDMessage("title", "file-abc123")

// Legacy functions (deprecated but still supported)
llm.NewUserFileMessage("filename.pdf", "data:application/pdf;base64,...")
llm.NewUserFileIDMessage("file-abc123")
```

## Supported Formats

Both providers support:
- **PDF files** (primary focus)
- **Base64 encoded data**
- **URL references** to publicly accessible documents
- **Files API references** (provider-specific file IDs)

## Requirements

### Environment Variables

- `ANTHROPIC_API_KEY`: Required for Anthropic provider
- `OPENAI_API_KEY`: Required for OpenAI Responses API provider

### Supported Models

#### Anthropic
- Claude Opus 4 (`claude-opus-4-20250514`)
- Claude Sonnet 4 (`claude-sonnet-4-20250514`)
- Claude Sonnet 3.7 (`claude-3-7-sonnet-20250219`)
- Claude Sonnet 3.5 models
- Claude Haiku 3.5 (`claude-3-5-haiku-20241022`)

#### OpenAI Responses API
- GPT-4o and GPT-4o-mini models with PDF support

## Limitations

- **File Size**: Maximum 32MB per request
- **Page Limit**: Maximum 100 pages per PDF
- **Format**: Standard PDFs only (no password protection or encryption)
- **Token Costs**: PDFs consume both text and image tokens (each page is converted to an image)

## Examples

### Basic Analysis

```bash
go run main.go -url "https://assets.anthropic.com/m/1cd9d098ac3e6467/original/Claude-3-Model-Card-October-Addendum.pdf"
```

### Custom Analysis

```bash
go run main.go -pdf "./financial-report.pdf" -prompt "Extract the quarterly revenue figures and growth percentages"
```

### Debugging

```bash
go run main.go -pdf "./document.pdf" -log debug -prompt "Analyze this document"
``` 