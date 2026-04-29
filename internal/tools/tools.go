// Package tools ensures we include the tool names in the context window.
package tools

import (
	"context"
	"encoding/json"

	"github.com/invopop/jsonschema"
)

const (
	ToolNameBash        = "bash"
	ToolNameReadFile    = "read_file"
	ToolNameEditFile    = "edit_file"
	ToolNameGrepSearch  = "grep_search"
	ToolNameListFiles   = "list_files"
	ToolNameFinder      = "finder"
	ToolNameWebSearch   = "web_search"
	ToolNameReadWebPage = "read_web_page"
)

// ToolDefinition represents a tool that can be called by the model
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema *jsonschema.Schema
	Function    ToolRunnerFunc
}

// ToolRunner defines an interface for executing a tool
type ToolRunner interface {
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

// ToolRunnerFunc allows functions to implement ToolRunner
type ToolRunnerFunc func(ctx context.Context, args json.RawMessage) (string, error)

func (f ToolRunnerFunc) Run(ctx context.Context, args json.RawMessage) (string, error) {
	return f(ctx, args)
}

// ToolCapable is an optional interface that models can implement
// to receive a ToolExecutor for handling tool calls.
type ToolCapable interface {
	SetToolExecutor(ToolExecutor)
}

// ToolExecutor can execute tools by name and provide access to tool definitions
type ToolExecutor interface {
	ExecuteTool(ctx context.Context, name string, args json.RawMessage) (string, error)
	GetRegisteredTools() []ToolDefinition
}
