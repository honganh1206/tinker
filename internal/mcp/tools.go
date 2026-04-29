package mcp

import (
	"github.com/invopop/jsonschema"
)

// ToolResultContent defines the structure for content returned by a tool call.
type ToolResultContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// Tool defines the structure for a tool's metadata.
type Tool struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	InputSchema *jsonschema.Schema `json:"inputSchema"`
}
