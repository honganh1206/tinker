package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
)

type AnthropicClient struct {
	client       *anthropic.Client
	model        ModelVersion
	maxTokens    int64
	cache        anthropic.CacheControlEphemeralParam
	systemPrompt string
}

func NewAnthropicClient(client *anthropic.Client, model ModelVersion, maxTokens int64, systemPrompt string) *AnthropicClient {
	return &AnthropicClient{
		client:       client,
		model:        model,
		maxTokens:    maxTokens,
		cache:        anthropic.NewCacheControlEphemeralParam(),
		systemPrompt: systemPrompt,
	}
}

func (c *AnthropicClient) Provider() string {
	return AnthropicProvider
}

func (c *AnthropicClient) Model() string {
	return string(c.model)
}

func (c *AnthropicClient) Generate(ctx context.Context, req Request) (*message.Message, error) {
	history, err := toAnthropicHistory(req.Messages)
	if err != nil {
		return nil, err
	}

	if len(history) == 0 {
		return nil, errors.New("anthropic: no messages in conversation history")
	}

	nativeTools, err := toAnthropicTools(req.Tools)
	if err != nil {
		return nil, err
	}

	sysPrompt := c.systemPrompt
	if req.SystemPrompt != "" {
		sysPrompt = req.SystemPrompt
	}

	maxTokens := c.maxTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	params := anthropic.MessageNewParams{
		Model:     getModel(c.model),
		MaxTokens: maxTokens,
		Messages:  history,
		Tools:     nativeTools,
		System: []anthropic.TextBlockParam{
			{Text: sysPrompt, CacheControl: c.cache},
		},
	}

	response, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic call failed: %w", err)
	}

	return toGenericMessage(*response)
}

func (c *AnthropicClient) CountTokens(ctx context.Context, req Request) (int, error) {
	history, err := toAnthropicHistory(req.Messages)
	if err != nil {
		return 0, err
	}

	sysPrompt := c.systemPrompt
	if req.SystemPrompt != "" {
		sysPrompt = req.SystemPrompt
	}

	count, err := c.client.Messages.CountTokens(ctx, anthropic.MessageCountTokensParams{
		Messages: history,
		Model:    getModel(c.model),
		System: anthropic.MessageCountTokensParamsSystemUnion{
			OfTextBlockArray: []anthropic.TextBlockParam{
				{Text: sysPrompt, CacheControl: c.cache},
			},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("anthropic token count failed: %w", err)
	}

	return int(count.InputTokens), nil
}

func getModel(model ModelVersion) anthropic.Model {
	switch model {
	case Claude46Opus:
		return anthropic.ModelClaudeOpus4_6
	case Claude45Opus:
		return anthropic.ModelClaudeOpus4_5_20251101
	case Claude41Opus:
		return anthropic.ModelClaudeOpus4_1_20250805
	case Claude4Opus:
		return anthropic.ModelClaudeOpus4_0
	case Claude46Sonnet:
		return anthropic.ModelClaudeSonnet4_6
	case Claude45Sonnet:
		return anthropic.ModelClaudeSonnet4_5
	case Claude4Sonnet:
		return anthropic.ModelClaudeSonnet4_0
	case Claude45Haiku:
		return anthropic.ModelClaudeHaiku4_5
	case Claude35Haiku:
		return "claude-3-5-haiku-latest"
	case Claude3Opus:
		return "claude-3-opus-20240229"
	case Claude3Haiku:
		return anthropic.ModelClaude_3_Haiku_20240307
	default:
		return anthropic.ModelClaudeSonnet4_0
	}
}

func toAnthropicHistory(messages []*message.Message) ([]anthropic.MessageParam, error) {
	history := make([]anthropic.MessageParam, 0, len(messages))

	for _, msg := range messages {
		if msg == nil {
			return nil, errors.New("anthropic: message is nil")
		}

		blocks := toBlocks(msg.Content)
		var nativeMsg anthropic.MessageParam
		switch msg.Role {
		case message.UserRole:
			nativeMsg = anthropic.NewUserMessage(blocks...)
		case message.AssistantRole:
			nativeMsg = anthropic.NewAssistantMessage(blocks...)
		default:
			return nil, fmt.Errorf("anthropic: invalid message role: %s", msg.Role)
		}

		history = append(history, nativeMsg)
	}

	return history, nil
}

func toAnthropicTools(toolDefs []*tools.ToolDefinition) ([]anthropic.ToolUnionParam, error) {
	if len(toolDefs) == 0 {
		return nil, nil
	}

	nativeTools := make([]anthropic.ToolUnionParam, 0, len(toolDefs))

	for _, tool := range toolDefs {
		anthropicTool, err := toAnthropicTool(tool)
		if err != nil {
			return nil, err
		}
		nativeTools = append(nativeTools, anthropicTool)
	}

	return nativeTools, nil
}

func toBlocks(blocks []message.ContentBlock) []anthropic.ContentBlockParamUnion {
	anthropicBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(blocks))

	for _, block := range blocks {
		switch b := block.(type) {
		case message.ToolResultBlock:
			anthropicBlocks = append(anthropicBlocks, anthropic.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError))
		case message.TextBlock:
			anthropicBlocks = append(anthropicBlocks, anthropic.NewTextBlock(b.Text))
		case message.ToolUseBlock:
			toolUseParam := anthropic.ToolUseBlockParam{
				ID:    b.ID,
				Name:  b.Name,
				Input: b.Input,
			}
			anthropicBlocks = append(anthropicBlocks, anthropic.ContentBlockParamUnion{
				OfToolUse: &toolUseParam,
			})
		}
	}

	return anthropicBlocks
}

func toGenericMessage(anthropicMsg anthropic.Message) (*message.Message, error) {
	msg := &message.Message{
		Role:    message.AssistantRole,
		Content: make([]message.ContentBlock, 0),
	}

	for _, block := range anthropicMsg.Content {
		switch variant := block.AsAny().(type) {
		case anthropic.TextBlock:
			msg.Content = append(msg.Content, message.NewTextBlock(block.Text))
		case anthropic.ToolUseBlock:
			err := json.Unmarshal([]byte(variant.JSON.Input.Raw()), &block.Input)
			if err != nil {
				return nil, err
			}
			msg.Content = append(msg.Content, message.NewToolUseBlock(block.ID, block.Name, block.Input))
		}
	}

	return msg, nil
}

func toAnthropicTool(tool *tools.ToolDefinition) (anthropic.ToolUnionParam, error) {
	schema, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return anthropic.ToolUnionParam{}, fmt.Errorf("failed to marshal tool schema: %w", err)
	}

	var anthropicSchema anthropic.ToolInputSchemaParam
	if err := json.Unmarshal(schema, &anthropicSchema); err != nil {
		return anthropic.ToolUnionParam{}, fmt.Errorf("failed to unmarshal to Anthropic schema: %w", err)
	}

	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
			InputSchema: anthropicSchema,
		},
	}, nil
}
