package model

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/honganh1206/tinker/internal/storage"
	"github.com/honganh1206/tinker/internal/tools"
)

const (
	// Claude
	Claude46Opus   ModelVersion = "claude-opus-4-6"
	Claude46Sonnet ModelVersion = "claude-sonnet-4-6"
	Claude45Haiku  ModelVersion = "claude-haiku-4-5"
	Claude45Opus   ModelVersion = "claude-opus-4-5"
	Claude45Sonnet ModelVersion = "claude-sonnet4-5"
	Claude41Opus   ModelVersion = "claude-opus-4-1"
	Claude4Opus    ModelVersion = "claude-opus-4"
	Claude4Sonnet  ModelVersion = "claude-sonnet-4"
)

type ClaudeModel struct {
	client       *anthropic.Client
	model        ModelVersion
	toolExecutor tools.ToolExecutor
	cache        anthropic.CacheControlEphemeralParam
}

func NewClaudeModel(model ModelVersion) (*ClaudeModel, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeModel{
		client: &client,
		model:  model,
		cache:  anthropic.NewCacheControlEphemeralParam(),
	}, nil
}

func (c *ClaudeModel) MaxTokens() int {
	return 300_000
}

// SetToolExecutor sets the tool executor for the Claude model
func (c *ClaudeModel) SetToolExecutor(executor tools.ToolExecutor) {
	c.toolExecutor = executor
}

func (c *ClaudeModel) Call(ctx context.Context, inputs []storage.Record) ([]storage.Record, int, error) {
	var availableTools []tools.ToolDefinition
	if c.toolExecutor != nil {
		availableTools = c.toolExecutor.GetRegisteredTools()
	}

	var systemBlocks []anthropic.TextBlockParam
	var messages []anthropic.MessageParam

	for _, rec := range inputs {
		switch rec.Source {
		case storage.SystemPrompt:
			systemBlocks = append(systemBlocks, anthropic.TextBlockParam{
				Text:         rec.Content,
				CacheControl: c.cache,
			})
		case storage.Prompt:
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(rec.Content)))
		case storage.ModelResp:
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(rec.Content)))
		case storage.ToolResult:
			// Store raw content in a message,
			// not really efficient so there should be a better solution
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(rec.Content),
			))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: 4096,
		Messages:  messages,
	}

	if len(systemBlocks) > 0 {
		params.System = systemBlocks
	}

	if len(availableTools) > 0 {
		tools := getClaudeToolParams(availableTools)
		params.Tools = tools
	}

	// Send the user prompt
	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("claude api: %w", err)
	}

	var events []storage.Record
	totalTokens := int(resp.Usage.InputTokens + resp.Usage.OutputTokens)

	for hasToolUse(resp.Content) {
		var assistantContent []anthropic.ContentBlockParamUnion

		for _, block := range resp.Content {
			if block.Type == "text" && block.Text != "" {
				assistantContent = append(assistantContent, anthropic.NewTextBlock(block.Text))
			} else if block.Type == "tool_use" {
				assistantContent = append(assistantContent, anthropic.NewToolUseBlock(block.ID, block.Input, block.Name))
			}
		}

		messages = append(messages, anthropic.MessageParam{
			Role:    anthropic.MessageParamRoleAssistant,
			Content: assistantContent,
		})

		var toolResults []anthropic.ContentBlockParamUnion

		// Execute the tools that the LLM chooses
		for _, block := range resp.Content {
			if block.Type == "tool_use" {
				inputStr := string(block.Input)
				// Middlewares go here
				// but since we don't have any use case for it yet
				out, err := c.toolExecutor.ExecuteTool(ctx, block.Name, block.Input)
				if err != nil {
					out = fmt.Sprintf("error executing tool: %v", err)
				}

				call := fmt.Sprintf("%s(%s)", block.Name, inputStr)
				events = append(events, storage.Record{
					Source:    storage.ToolUse,
					Content:   call,
					Live:      true,
					EstTokens: storage.TokenCount(call),
				})
				isErr := err != nil
				toolResults = append(toolResults, anthropic.NewToolResultBlock(block.ID, out, isErr))
			}
		}

		// Send the result back to the LLM
		// and continue using the next tools
		messages = append(messages, anthropic.NewUserMessage(toolResults...))

		params.Messages = messages
		resp, err = c.client.Messages.New(ctx, params)
		if err != nil {
			return nil, 0, fmt.Errorf("claude api (tool continuation): %w", err)
		}

		totalTokens += int(resp.Usage.InputTokens + resp.Usage.OutputTokens)
	}

	// Final response from the LLM
	var responseText string
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			responseText += block.Text
		}
	}

	events = append(events, storage.Record{
		Source:    storage.ModelResp,
		Content:   responseText,
		Live:      true,
		EstTokens: storage.TokenCount(responseText),
	})

	return events, totalTokens, nil
}

func hasToolUse(content []anthropic.ContentBlockUnion) bool {
	for _, block := range content {
		if block.Type == "tool_use" {
			return true
		}
	}
	return false
}

// NOTE: This only supports built-in tools for now
// so we should handle custom tools with ToolBuilder struct
func getClaudeToolParams(availableTools []tools.ToolDefinition) []anthropic.ToolUnionParam {
	var toolParams []anthropic.ToolUnionParam
	for _, tool := range availableTools {
		schema, err := json.Marshal(tool.InputSchema)
		if err != nil {
			panic(fmt.Sprintf("cannot marshal unified tool schema: %v", err))
		}

		var anthropicSchema anthropic.ToolInputSchemaParam
		if err := json.Unmarshal(schema, &anthropicSchema); err != nil {
			panic(fmt.Sprintf("cannot unmarshal tool definition to Claude format: %v", err))
		}

		toolParams = append(toolParams, anthropic.ToolUnionParamOfTool(anthropicSchema, tool.Name))
	}
	return toolParams
}
