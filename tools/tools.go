package tools

import (
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

type ToolBox struct {
	Tools []*ToolDefinition
}

type ToolDefinition struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	InputSchema *jsonschema.Schema `json:"input_schema"`
	Function    func(input ToolInput) (string, error)
}

type ToolInput struct {
	RawInput json.RawMessage
}
