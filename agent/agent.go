package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
)

type Agent struct {
	LLM     inference.LLMClient
	ToolBox *tools.ToolBox
	Conv    *message.Conversation
	MCP     mcp.Config
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

	agent := &Agent{
		LLM:     config.LLM,
		ToolBox: config.ToolBox,
		Conv:    config.Conversation,
		Logger:  logger,
	}

	agent.MCP.ServerConfigs = config.MCPConfigs
	agent.MCP.ActiveServers = []*mcp.Server{}
	agent.MCP.Tools = []mcp.Tools{}
	agent.MCP.ToolMap = make(map[string]mcp.ToolDetails)

	return agent
}

// Run handles a single user message and returns the agent's response
func (a *Agent) Run(ctx context.Context, userInput string, onDelta func(string)) error {
	readUserInput := true

	// TODO: Add flag to know when to summarize
	a.Conv.Messages = a.LLM.SummarizeHistory(a.Conv.Messages, 20)

	if len(a.Conv.Messages) != 0 {
		a.LLM.ToNativeHistory(a.Conv.Messages)
	}

	a.LLM.ToNativeTools(a.ToolBox.Tools)

	for {
		if readUserInput {
			userMsg := &message.Message{
				Role:    message.UserRole,
				Content: []message.ContentBlock{message.NewTextBlock(userInput)},
			}

			err := a.LLM.ToNativeMessage(userMsg)
			if err != nil {
				return err
			}

			a.Conv.Append(userMsg)
		}

		agentMsg, err := a.streamResponse(ctx, onDelta)
		if err != nil {
			return err
		}

		err = a.LLM.ToNativeMessage(agentMsg)
		if err != nil {
			return err
		}

		a.Conv.Append(agentMsg)

		toolResults := []message.ContentBlock{}
		for _, c := range agentMsg.Content {
			switch block := c.(type) {
			case message.ToolUseBlock:
				result := a.executeTool(block.ID, block.Name, block.Input, onDelta)
				toolResults = append(toolResults, result)
			}
		}

		if len(toolResults) == 0 {
			readUserInput = true
			count, err := a.LLM.CountTokens(ctx)
			if err != nil {
				return err
			}
			a.Conv.TokenCount = count
			break
		}

		readUserInput = false

		toolResultMsg := &message.Message{
			Role:    message.UserRole,
			Content: toolResults,
		}

		err = a.LLM.ToNativeMessage(toolResultMsg)
		if err != nil {
			return err
		}

		a.Conv.Append(toolResultMsg)
	}
	return nil
}

func (a *Agent) executeTool(id, name string, input json.RawMessage, onDelta func(string)) message.ContentBlock {
	var result message.ContentBlock
	if execDetails, isMCPTool := a.MCP.ToolMap[name]; isMCPTool {
		result = a.executeMCPTool(id, name, input, execDetails)
	} else {
		result = a.executeLocalTool(id, name, input)
	}

	isError := false
	if toolResult, ok := result.(message.ToolResultBlock); ok && toolResult.IsError {
		isError = true
	}
	a.Logger.Info("tool executed", "name", name, "error", isError)

	return result
}

func (a *Agent) executeMCPTool(id, name string, input json.RawMessage, toolDetails mcp.ToolDetails) message.ContentBlock {
	var args map[string]any

	err := json.Unmarshal(input, &args)
	if err != nil {
		// TODO: No error handling here
	}
	if args == nil {
		// This is kinda dumb?
		args = make(map[string]any)
	}

	result, err := toolDetails.Server.Call(context.Background(), name, args)
	if err != nil {
		return message.NewToolResultBlock(id, name,
			fmt.Sprintf("MCP tool %s execution error: %v", name, err), true)
	}
	if result == nil {
		return message.NewToolResultBlock(id, name, "Tool executed successfully but returned no content", false)
	}

	// We have to do this,
	// otherwise there will be an error saying
	// "all messages must have non-empty content etc."
	// even though we do have :)
	content := fmt.Sprintf("%v", result)
	if content == "" {
		return message.NewToolResultBlock(id, name, "Tool executed successfully but returned empty content", false)
	}

	return message.NewToolResultBlock(id, name, content, false)
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

func (a *Agent) streamResponse(ctx context.Context, onDelta func(string)) (*message.Message, error) {
	var streamErr error
	var msg *message.Message

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		msg, streamErr = a.LLM.RunInference(ctx, onDelta, true)
	}()

	wg.Wait()

	if streamErr != nil {
		return nil, streamErr
	}

	return msg, nil
}
