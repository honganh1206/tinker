package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/honganh1206/tinker/internal/mcp"
)

// MCPToolRunner adapts an MCP manager call into the tools.ToolRunner interface.
type MCPToolRunner struct {
	Manager *mcp.Manager
	Name    string
}

func (r *MCPToolRunner) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return "", fmt.Errorf("parse MCP tool input: %w", err)
	}
	if params == nil {
		params = make(map[string]any)
	}

	result, err := r.Manager.Call(ctx, r.Name, params)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%v", result), nil
}
