# MCP Enhanced Content Type Support

This document describes the enhanced content type support in Dive's MCP integration, which provides comprehensive handling of all MCP content types and proper annotation conversion.

## Supported Content Types

### 1. Text Content (`*mcp.TextContent`)
- **Dive Type**: `dive.ToolResultContentTypeText`
- **Mapping**: Direct text content transfer
- **Annotations**: Converted to Dive annotations format

### 2. Image Content (`*mcp.ImageContent`)
- **Dive Type**: `dive.ToolResultContentTypeImage`
- **Mapping**: Base64 data and MIME type preserved
- **Annotations**: Converted to Dive annotations format

### 3. Audio Content (`*mcp.AudioContent`)
- **Dive Type**: `dive.ToolResultContentTypeAudio`
- **Mapping**: Base64 data and MIME type preserved
- **Annotations**: Converted to Dive annotations format

### 4. Embedded Resources (`*mcp.EmbeddedResource`)

#### Text Resources (`*mcp.TextResourceContents`)
- **Dive Type**: `dive.ToolResultContentTypeText`
- **Mapping**: Resource text content transferred
- **Annotations**: Resource metadata added (URI, MIME type)

#### Blob Resources (`*mcp.BlobResourceContents`)
- **Dive Type**: `dive.ToolResultContentTypeText`
- **Mapping**: Human-readable description of binary resource
- **Annotations**: Resource metadata added (URI, MIME type, type)

### 5. Unknown/Future Content Types
- **Dive Type**: `dive.ToolResultContentTypeText`
- **Mapping**: JSON serialization with reflection-based field extraction
- **Fallback**: Graceful handling for extensibility

## Annotation Conversion

MCP annotations are converted to Dive's annotation format:

| MCP Annotation | Dive Annotation Key | Description              |
| -------------- | ------------------- | ------------------------ |
| `Priority`     | `mcp_priority`      | Priority level (0.0-1.0) |
| `Audience`     | `mcp_audience`      | Array of audience roles  |

For embedded resources, additional metadata is preserved:
- `mcp_resource_uri`: Original resource URI
- `mcp_resource_mime_type`: Resource MIME type
- `mcp_resource_type`: Resource type (text/blob)

## Usage Example

```go
// MCP tool returns mixed content
mcpResult := &mcp.CallToolResult{
    Content: []mcp.Content{
        &mcp.TextContent{
            Type: "text",
            Text: "Analysis complete:",
            Annotated: mcp.Annotated{
                Annotations: &mcp.Annotations{
                    Priority: 0.8,
                    Audience: []mcp.Role{mcp.RoleUser},
                },
            },
        },
        &mcp.ImageContent{
            Type:     "image",
            Data:     "iVBORw0KGgoAAAA...",
            MIMEType: "image/png",
        },
        &mcp.EmbeddedResource{
            Type: "resource",
            Resource: &mcp.TextResourceContents{
                URI:      "file:///report.txt",
                MIMEType: "text/plain",
                Text:     "Detailed analysis results...",
            },
        },
    },
}

// Automatically converted by MCP adapter
diveResult, err := mcp.ConvertMCPResultToDive(mcpResult)
// diveResult.Content contains properly typed and annotated content
```

## Key Features

1. **Complete Coverage**: Handles all current MCP content types
2. **Annotation Preservation**: MCP metadata preserved and accessible
3. **Type Safety**: Proper mapping to Dive's type system
4. **Future Compatibility**: Graceful fallback for unknown types
5. **Resource Handling**: Smart conversion of embedded resources
6. **Multi-Content**: Support for complex responses with multiple content items

## Benefits for Dive Users

- **Rich Content**: Full support for multimedia and document content from MCP tools
- **Metadata Access**: Complete annotation and resource metadata available
- **Consistent API**: Uniform content handling regardless of MCP server complexity
- **Future-Proof**: Automatic handling of new MCP content types 