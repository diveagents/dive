package main

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/diveagents/dive"
	mcpAdapter "github.com/diveagents/dive/mcp"
	"github.com/mark3labs/mcp-go/mcp"
)

// simulateGetImage creates a mock MCP tool result with image content
func simulateGetImage() *mcp.CallToolResult {
	// Create a simple 1x1 PNG image (base64 encoded)
	pngData := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{
				Type:     "image",
				Data:     pngData,
				MIMEType: "image/png",
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{
						Audience: []mcp.Role{mcp.RoleUser},
						Priority: 0.9,
					},
				},
			},
		},
		IsError: false,
	}
}

// simulateGetAudio creates a mock MCP tool result with audio content
func simulateGetAudio() *mcp.CallToolResult {
	// Create a simple WAV file header (base64 encoded)
	wavData := "UklGRiQAAABXQVZFZm10IBAAAAABAAEAgLsAAADuAgAEABAAZGF0YQAAAAAA"

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.AudioContent{
				Type:     "audio",
				Data:     wavData,
				MIMEType: "audio/wav",
			},
		},
		IsError: false,
	}
}

// simulateGetDocument creates a mock MCP tool result with embedded resource
func simulateGetDocument() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Type: "resource",
				Resource: &mcp.TextResourceContents{
					URI:      "file:///path/to/document.txt",
					MIMEType: "text/plain",
					Text:     "This is the content of an important document that was retrieved from the file system.",
				},
				Annotated: mcp.Annotated{
					Annotations: &mcp.Annotations{
						Audience: []mcp.Role{mcp.RoleUser, mcp.RoleAssistant},
						Priority: 0.7,
					},
				},
			},
		},
		IsError: false,
	}
}

// simulateGetBinaryFile creates a mock MCP tool result with binary resource
func simulateGetBinaryFile() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Type: "resource",
				Resource: &mcp.BlobResourceContents{
					URI:      "file:///path/to/data.bin",
					MIMEType: "application/octet-stream",
					Blob:     base64.StdEncoding.EncodeToString([]byte("Binary data here")),
				},
			},
		},
		IsError: false,
	}
}

func main() {
	fmt.Println("=== MCP Enhanced Content Types Demo ===\n")

	// Example 1: Image Content
	fmt.Println("1. Image Content:")
	imageResult := simulateGetImage()
	diveImageResult, err := convertMCPResult(imageResult)
	if err != nil {
		log.Fatal(err)
	}
	printToolResult("Generated Image", diveImageResult)

	// Example 2: Audio Content
	fmt.Println("\n2. Audio Content:")
	audioResult := simulateGetAudio()
	diveAudioResult, err := convertMCPResult(audioResult)
	if err != nil {
		log.Fatal(err)
	}
	printToolResult("Generated Audio", diveAudioResult)

	// Example 3: Text Document Resource
	fmt.Println("\n3. Text Document Resource:")
	docResult := simulateGetDocument()
	diveDocResult, err := convertMCPResult(docResult)
	if err != nil {
		log.Fatal(err)
	}
	printToolResult("Retrieved Document", diveDocResult)

	// Example 4: Binary File Resource
	fmt.Println("\n4. Binary File Resource:")
	binaryResult := simulateGetBinaryFile()
	diveBinaryResult, err := convertMCPResult(binaryResult)
	if err != nil {
		log.Fatal(err)
	}
	printToolResult("Retrieved Binary", diveBinaryResult)

	// Example 5: Multiple Content Types
	fmt.Println("\n5. Multiple Content Types:")
	multiResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Type: "text",
				Text: "Here's what I found:",
			},
			&mcp.ImageContent{
				Type:     "image",
				Data:     "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
				MIMEType: "image/png",
			},
			&mcp.TextContent{
				Type: "text",
				Text: "And here's the accompanying document:",
			},
			&mcp.EmbeddedResource{
				Type: "resource",
				Resource: &mcp.TextResourceContents{
					URI:      "file:///summary.txt",
					MIMEType: "text/plain",
					Text:     "Summary: The analysis is complete.",
				},
			},
		},
		IsError: false,
	}
	diveMultiResult, err := convertMCPResult(multiResult)
	if err != nil {
		log.Fatal(err)
	}
	printToolResult("Multi-Content Response", diveMultiResult)
}

// Helper function to use the internal converter (this would normally be done by the MCP adapter)
func convertMCPResult(mcpResult *mcp.CallToolResult) (*dive.ToolResult, error) {
	// This simulates what the MCPToolAdapter does internally
	// In real usage, this conversion happens automatically when calling MCP tools
	return mcpAdapter.ConvertMCPResultToDive(mcpResult)
}

func printToolResult(title string, result *dive.ToolResult) {
	fmt.Printf("Tool Result: %s\n", title)
	fmt.Printf("  Error: %t\n", result.IsError)
	fmt.Printf("  Content Items: %d\n", len(result.Content))

	for i, content := range result.Content {
		fmt.Printf("  [%d] Type: %s\n", i+1, content.Type)

		switch content.Type {
		case dive.ToolResultContentTypeText:
			fmt.Printf("      Text: %q\n", truncateString(content.Text, 60))
		case dive.ToolResultContentTypeImage:
			fmt.Printf("      MIME: %s\n", content.MimeType)
			fmt.Printf("      Data: %s... (%d chars)\n", content.Data[:min(20, len(content.Data))], len(content.Data))
		case dive.ToolResultContentTypeAudio:
			fmt.Printf("      MIME: %s\n", content.MimeType)
			fmt.Printf("      Data: %s... (%d chars)\n", content.Data[:min(20, len(content.Data))], len(content.Data))
		}

		if content.Annotations != nil && len(content.Annotations) > 0 {
			fmt.Printf("      Annotations: %v\n", content.Annotations)
		}
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
