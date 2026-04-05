package inference

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/schema"
	"github.com/honganh1206/tinker/tools"
	"google.golang.org/genai"
)

type GeminiClient struct {
	client       *genai.Client
	model        ModelVersion
	maxTokens    int64
	systemPrompt string
}

func NewGeminiClient(client *genai.Client, model ModelVersion, maxTokens int64, systemPrompt string) *GeminiClient {
	return &GeminiClient{
		client:       client,
		model:        model,
		maxTokens:    maxTokens,
		systemPrompt: systemPrompt,
	}
}

func (c *GeminiClient) Provider() string {
	return GoogleProvider
}

func (c *GeminiClient) Model() string {
	return string(c.model)
}

func (c *GeminiClient) Generate(ctx context.Context, req Request) (*message.Message, error) {
	contents, err := toGeminiHistory(req.Messages)
	if err != nil {
		return nil, err
	}

	if len(contents) == 0 {
		return nil, errors.New("gemini: no messages in conversation history")
	}

	nativeTools, err := toGeminiTools(req.Tools)
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

	config := &genai.GenerateContentConfig{
		MaxOutputTokens:   int32(maxTokens),
		Tools:             nativeTools,
		SystemInstruction: genai.NewContentFromText(sysPrompt, genai.RoleUser),
	}

	response, err := c.client.Models.GenerateContent(ctx, string(c.model), contents, config)
	if err != nil {
		return nil, fmt.Errorf("gemini call failed: %w", err)
	}

	if len(response.Candidates) == 0 || response.Candidates[0].Content == nil {
		return nil, fmt.Errorf("gemini: no content returned")
	}

	bestContent := response.Candidates[0].Content

	msg := &message.Message{
		Role:    message.AssistantRole,
		Content: make([]message.ContentBlock, 0),
	}

	var fullText strings.Builder
	var blocks []message.ContentBlock

	for _, p := range bestContent.Parts {
		if p.Text != "" {
			fullText.WriteString(p.Text)
		}
		if p.FunctionCall != nil {
			fc := p.FunctionCall
			input, err := json.Marshal(fc.Args)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal function args: %w", err)
			}

			var thought json.RawMessage
			if p.ThoughtSignature != nil {
				ts, err := json.Marshal(p.ThoughtSignature)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal thought: %w", err)
				}
				thought = ts
			}

			blocks = append(blocks, message.ToolUseBlock{
				ID:      fc.ID,
				Name:    fc.Name,
				Input:   input,
				Thought: thought,
			})
		}
	}

	if fullText.Len() > 0 {
		msg.Content = append(msg.Content, message.NewTextBlock(fullText.String()))
	}

	msg.Content = append(msg.Content, blocks...)

	return msg, nil
}

func (c *GeminiClient) CountTokens(ctx context.Context, req Request) (int, error) {
	contents, err := toGeminiHistory(req.Messages)
	if err != nil {
		return 0, err
	}

	count, err := c.client.Models.CountTokens(ctx, string(c.model), contents, nil)
	if err != nil {
		return 0, fmt.Errorf("gemini token count failed: %w", err)
	}

	return int(count.TotalTokens), nil
}

// Pure conversion functions — no client state mutation

func toGeminiHistory(messages []*message.Message) ([]*genai.Content, error) {
	contents := make([]*genai.Content, 0, len(messages))

	for _, msg := range messages {
		if msg == nil {
			return nil, errors.New("gemini: message is nil")
		}

		parts, err := toParts(msg.Content)
		if err != nil {
			return nil, err
		}

		if len(parts) == 0 {
			return nil, errors.New("gemini: message has no content parts")
		}

		role := msg.Role
		if role == message.AssistantRole {
			role = "model"
		}

		contents = append(contents, &genai.Content{
			Role:  role,
			Parts: parts,
		})
	}

	return contents, nil
}

func toGeminiTools(toolDefs []*tools.ToolDefinition) ([]*genai.Tool, error) {
	if len(toolDefs) == 0 {
		return nil, nil
	}

	builtinTool := &genai.Tool{
		FunctionDeclarations: make([]*genai.FunctionDeclaration, 0, len(toolDefs)),
	}

	for _, tool := range toolDefs {
		funcDecl, err := toFunctionDeclaration(tool)
		if err != nil {
			return nil, err
		}
		builtinTool.FunctionDeclarations = append(builtinTool.FunctionDeclarations, funcDecl)
	}

	return []*genai.Tool{builtinTool}, nil
}

func toParts(blocks []message.ContentBlock) ([]*genai.Part, error) {
	parts := make([]*genai.Part, 0, len(blocks))

	for _, block := range blocks {
		switch b := block.(type) {
		case message.TextBlock:
			if b.Text != "" {
				parts = append(parts, genai.NewPartFromText(b.Text))
			}
		case message.ToolUseBlock:
			var args map[string]any
			if err := json.Unmarshal(b.Input, &args); err != nil {
				return nil, fmt.Errorf("gemini: failed to unmarshal tool input: %w", err)
			}

			part := genai.NewPartFromFunctionCall(b.Name, args)

			if len(b.Thought) > 0 {
				var sig []byte
				if err := json.Unmarshal(b.Thought, &sig); err == nil && len(sig) > 0 {
					part.ThoughtSignature = sig
				}
			}

			parts = append(parts, part)
		case message.ToolResultBlock:
			response := map[string]any{"result": b.Content}
			parts = append(parts, genai.NewPartFromFunctionResponse(b.ToolName, response))
		}
	}

	return parts, nil
}

func toFunctionDeclaration(tool *tools.ToolDefinition) (*genai.FunctionDeclaration, error) {
	params, err := schema.ConvertToGeminiSchema(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema to Gemini format: %w", err)
	}

	return &genai.FunctionDeclaration{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  params,
	}, nil
}
