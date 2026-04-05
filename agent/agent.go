package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
)

type Agent struct {
	LLM     inference.LLMClient
	ToolBox *tools.ToolBox
	Conv    *message.Conversation
	MCP     *mcp.Manager
	Logger  *slog.Logger
}

type Config struct {
	LLM          inference.LLMClient
	Conversation *message.Conversation
	ToolBox      *tools.ToolBox
	MCPConfigs   []mcp.ServerConfig
	Logger       *slog.Logger
}

func New(config *Config) *Agent {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	a := &Agent{
		LLM:     config.LLM,
		ToolBox: config.ToolBox,
		Conv:    config.Conversation,
		Logger:  logger,
	}

	if len(config.MCPConfigs) > 0 {
		a.MCP = mcp.NewManager()
	}

	return a
}

func (a *Agent) StartMCP(ctx context.Context, configs []mcp.ServerConfig) error {
	if a.MCP == nil {
		return nil
	}

	exposed, err := a.MCP.Start(ctx, configs)
	if err != nil {
		return err
	}

	for _, t := range exposed {
		a.ToolBox.Tools = append(a.ToolBox.Tools, &tools.ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}

	return nil
}

// Run handles a single user message and returns the agent's response
func (a *Agent) Run(ctx context.Context, userInput string) error {
	userMsg := &message.Message{
		Role:    message.UserRole,
		Content: []message.ContentBlock{message.NewTextBlock(userInput)},
	}
	a.Conv.Append(userMsg)

	for {
		req := inference.Request{
			Messages: a.Conv.Messages,
			Tools:    a.ToolBox.Tools,
		}

		agentMsg, err := a.LLM.Generate(ctx, req)
		if err != nil {
			return err
		}

		a.Conv.Append(agentMsg)

		var toolResults []message.ContentBlock
		for _, c := range agentMsg.Content {
			if block, ok := c.(message.ToolUseBlock); ok {
				result := a.executeTool(ctx, block.ID, block.Name, block.Input)
				toolResults = append(toolResults, result)
			}
		}

		if len(toolResults) == 0 {
			count, err := a.LLM.CountTokens(ctx, req)
			if err != nil {
				return err
			}
			a.Conv.TokenCount = count
			break
		}

		toolResultMsg := &message.Message{
			Role:    message.UserRole,
			Content: toolResults,
		}
		a.Conv.Append(toolResultMsg)
	}
	return nil
}

func (a *Agent) executeTool(ctx context.Context, id, name string, input json.RawMessage) message.ContentBlock {
	if a.MCP != nil && a.MCP.HasTool(name) {
		var args map[string]any
		if err := json.Unmarshal(input, &args); err != nil {
			return message.NewToolResultBlock(id, name,
				fmt.Sprintf("failed to parse tool input: %v", err), true)
		}
		if args == nil {
			args = make(map[string]any)
		}

		result, err := a.MCP.Call(ctx, name, args)
		if err != nil {
			return message.NewToolResultBlock(id, name, err.Error(), true)
		}

		return message.NewToolResultBlock(id, name, fmt.Sprintf("%v", result), false)
	}

	return a.executeLocalTool(id, name, input)
}

func (a *Agent) executeLocalTool(id, name string, input json.RawMessage) message.ContentBlock {
	var toolDef *tools.ToolDefinition
	var found bool
	for _, tool := range a.ToolBox.Tools {
		if tool.Name == name {
			toolDef = tool
			found = true
			break
		}
	}

	if !found {
		return message.NewToolResultBlock(id, name, "tool not found", true)
	}

	toolInput := tools.ToolInput{
		RawInput: input,
	}

	toolOutput, err := toolDef.Function(toolInput)
	if err != nil {
		return message.NewToolResultBlock(id, name, err.Error(), true)
	}

	return message.NewToolResultBlock(id, name, string(toolOutput), false)
}

