package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/diveagents/dive"
	"github.com/diveagents/dive/schema"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPToolAdapter adapts an MCP tool to implement Dive's Tool interface
type MCPToolAdapter struct {
	mcpClient  *MCPClient
	toolInfo   mcp.Tool
	serverName string
}

// NewMCPToolAdapter creates a new MCP tool adapter
func NewMCPToolAdapter(client *MCPClient, tool mcp.Tool, serverName string) *MCPToolAdapter {
	return &MCPToolAdapter{
		mcpClient:  client,
		toolInfo:   tool,
		serverName: serverName,
	}
}

// Name returns the name of the MCP tool
func (t *MCPToolAdapter) Name() string {
	return t.toolInfo.Name
}

// Description returns the description of the MCP tool
func (t *MCPToolAdapter) Description() string {
	if t.toolInfo.Description != "" {
		return t.toolInfo.Description
	}
	return fmt.Sprintf("MCP tool %s from server %s", t.toolInfo.Name, t.serverName)
}

// Schema converts MCP tool input schema to Dive's schema format
func (t *MCPToolAdapter) Schema() *schema.Schema {
	if t.toolInfo.InputSchema.Type == "" {
		// Return empty object schema if no input schema is provided
		return &schema.Schema{
			Type:       "object",
			Properties: map[string]*schema.Property{},
		}
	}

	// Create a Schema from the MCP schema
	diveSchema := &schema.Schema{
		Type: schema.SchemaType(t.toolInfo.InputSchema.Type),
	}

	// Convert properties
	if t.toolInfo.InputSchema.Properties != nil {
		diveSchema.Properties = make(map[string]*schema.Property)
		for key, prop := range t.toolInfo.InputSchema.Properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				diveSchema.Properties[key] = convertMCPSchemaToDiv(propMap)
			}
		}
	}

	return diveSchema
}

// Annotations returns tool annotations
func (t *MCPToolAdapter) Annotations() *dive.ToolAnnotations {
	annotations := &dive.ToolAnnotations{
		Title: fmt.Sprintf("%s (MCP:%s)", t.toolInfo.Name, t.serverName),
	}

	// Set hints based on MCP tool properties if available
	// These would need to be extracted from the MCP tool definition
	// For now, we'll set conservative defaults
	annotations.ReadOnlyHint = false
	annotations.DestructiveHint = false
	annotations.IdempotentHint = false
	annotations.OpenWorldHint = true

	return annotations
}

// Call executes the MCP tool
func (t *MCPToolAdapter) Call(ctx context.Context, input any) (*dive.ToolResult, error) {
	// Convert input to map[string]interface{} for MCP
	var arguments map[string]interface{}

	switch v := input.(type) {
	case map[string]interface{}:
		arguments = v
	case json.RawMessage:
		// Handle empty JSON input
		if len(v) == 0 || string(v) == `""` {
			arguments = make(map[string]interface{})
		} else {
			if err := json.Unmarshal(v, &arguments); err != nil {
				return dive.NewToolResultError(fmt.Sprintf("Failed to unmarshal input: %v", err)), nil
			}
		}
	case string:
		// Handle empty string input
		if v == "" {
			arguments = make(map[string]interface{})
		} else {
			// Try to unmarshal as JSON
			if err := json.Unmarshal([]byte(v), &arguments); err != nil {
				return dive.NewToolResultError(fmt.Sprintf("Failed to unmarshal string input as JSON: %v", err)), nil
			}
		}
	default:
		// Marshal and unmarshal to convert to map[string]interface{}
		data, err := json.Marshal(input)
		if err != nil {
			return dive.NewToolResultError(fmt.Sprintf("Failed to marshal input: %v", err)), nil
		}
		if err := json.Unmarshal(data, &arguments); err != nil {
			return dive.NewToolResultError(fmt.Sprintf("Failed to unmarshal input: %v", err)), nil
		}
	}

	// Call the MCP tool
	result, err := t.mcpClient.CallTool(ctx, t.toolInfo.Name, arguments)
	if err != nil {
		return dive.NewToolResultError(fmt.Sprintf("MCP tool call failed: %v", err)), nil
	}

	// Convert MCP result to Dive ToolResult
	return convertMCPResultToDive(result)
}

// convertMCPSchemaToDiv converts MCP JSON Schema to Dive Property
func convertMCPSchemaToDiv(mcpSchema map[string]interface{}) *schema.Property {
	diveSchema := &schema.Property{}

	// Handle basic schema properties
	if schemaType, ok := mcpSchema["type"].(string); ok {
		diveSchema.Type = schema.SchemaType(schemaType)
	}

	if description, ok := mcpSchema["description"].(string); ok {
		diveSchema.Description = description
	}

	// Note: Property doesn't have Title field, skipping

	// Handle properties for object types
	if properties, ok := mcpSchema["properties"].(map[string]interface{}); ok {
		diveSchema.Properties = make(map[string]*schema.Property)
		for key, prop := range properties {
			if propMap, ok := prop.(map[string]interface{}); ok {
				diveSchema.Properties[key] = convertMCPSchemaToDiv(propMap)
			}
		}
	}

	// Handle required fields
	if required, ok := mcpSchema["required"].([]interface{}); ok {
		diveSchema.Required = make([]string, len(required))
		for i, req := range required {
			if reqStr, ok := req.(string); ok {
				diveSchema.Required[i] = reqStr
			}
		}
	}

	// Handle array items
	if items, ok := mcpSchema["items"].(map[string]interface{}); ok {
		diveSchema.Items = convertMCPSchemaToDiv(items)
	}

	// Handle enum values
	if enum, ok := mcpSchema["enum"].([]interface{}); ok {
		stringEnum := make([]string, len(enum))
		for i, val := range enum {
			if str, ok := val.(string); ok {
				stringEnum[i] = str
			}
		}
		diveSchema.Enum = stringEnum
	}

	// Handle additional properties
	if additionalProps, ok := mcpSchema["additionalProperties"]; ok {
		if boolVal, ok := additionalProps.(bool); ok {
			diveSchema.AdditionalProperties = &boolVal
		}
	}

	return diveSchema
}

// convertMCPResultToDive converts MCP CallToolResult to Dive ToolResult
func convertMCPResultToDive(mcpResult *mcp.CallToolResult) (*dive.ToolResult, error) {
	if mcpResult == nil {
		return dive.NewToolResultError("MCP tool returned nil result"), nil
	}

	var content []*dive.ToolResultContent

	// Convert MCP content to Dive content
	for _, mcpContent := range mcpResult.Content {
		diveContent := &dive.ToolResultContent{}

		// Handle different MCP content types
		switch c := mcpContent.(type) {
		case *mcp.TextContent:
			diveContent.Type = dive.ToolResultContentTypeText
			diveContent.Text = c.Text
			if c.Annotations != nil {
				// diveContent.Annotations = c.Annotations
			}
		case *mcp.ImageContent:
			diveContent.Type = dive.ToolResultContentTypeImage
			diveContent.Data = c.Data
			diveContent.MimeType = c.MIMEType
			if c.Annotations != nil {
				// diveContent.Annotations = c.Annotations
			}
		default:
			// Default to text for unknown types
			diveContent.Type = dive.ToolResultContentTypeText
			jsonBytes, _ := json.Marshal(mcpContent)
			diveContent.Text = string(jsonBytes)
		}

		content = append(content, diveContent)
	}

	// Check if this is an error result
	isError := mcpResult.IsError

	return &dive.ToolResult{
		Content: content,
		IsError: isError,
	}, nil
}
