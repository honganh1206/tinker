package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/honganh1206/tinker/inference"
	"github.com/honganh1206/tinker/logger"
	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
)

type mockLLMClient struct {
	mock.Mock
}

func (m *mockLLMClient) Generate(ctx context.Context, req inference.Request) (*message.Message, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*message.Message), args.Error(1)
}

func (m *mockLLMClient) CountTokens(ctx context.Context, req inference.Request) (int, error) {
	args := m.Called(ctx, req)
	return args.Int(0), args.Error(1)
}

func (m *mockLLMClient) Provider() string {
	return "mock"
}

func (m *mockLLMClient) Model() string {
	return "mock-model"
}

// Test helpers
func createTestAgent() (*Agent, *mockLLMClient) {
	mockLLM := &mockLLMClient{}
	conv := message.NewConversation()
	toolBox := &tools.ToolBox{
		Tools: []*tools.ToolDefinition{
			{
				Name:        "test_tool",
				Description: "A test tool",
				Function: func(input tools.ToolInput) (string, error) {
					return "test result", nil
				},
			},
		},
	}

	agent := New(&Config{
		LLM:          mockLLM,
		Conversation: conv,
		ToolBox:      toolBox,
		MCPConfigs:   []mcp.ServerConfig{},
		Logger:       logger.NewDefaultLogger(),
	})
	return agent, mockLLM
}

func createTestMessage(role string, text string) *message.Message {
	return &message.Message{
		Role:      role,
		Content:   []message.ContentBlock{message.NewTextBlock(text)},
		CreatedAt: time.Now(),
	}
}

// Tests
func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		mcpCount int
	}{
		{
			name:     "creates agent with MCP configs",
			mcpCount: 2,
		},
		{
			name:     "creates agent without MCP configs",
			mcpCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &mockLLMClient{}
			conv := message.NewConversation()
			toolBox := &tools.ToolBox{Tools: []*tools.ToolDefinition{}}

			mcpConfigs := make([]mcp.ServerConfig, tt.mcpCount)
			for i := 0; i < tt.mcpCount; i++ {
				mcpConfigs[i] = mcp.ServerConfig{
					ID:      "test-server",
					Command: "test-command",
				}
			}

			agent := New(&Config{
				LLM:          mockLLM,
				Conversation: conv,
				ToolBox:      toolBox,
				MCPConfigs:   mcpConfigs,
			})

			assert.NotNil(t, agent)
			assert.Equal(t, mockLLM, agent.LLM)
			assert.Equal(t, conv, agent.Conv)
			assert.Equal(t, toolBox, agent.ToolBox)
			assert.NotNil(t, agent.Logger)
			if tt.mcpCount > 0 {
				assert.NotNil(t, agent.MCP)
			} else {
				assert.Nil(t, agent.MCP)
			}
		})
	}
}

func TestAgent_Run_SimpleTextResponse(t *testing.T) {
	agent, mockLLM := createTestAgent()

	mockLLM.On("Generate", mock.Anything, mock.Anything).Return(
		createTestMessage(message.AssistantRole, "Hello, how can I help?"), nil)
	mockLLM.On("CountTokens", mock.Anything, mock.Anything).Return(0, nil).Once()

	err := agent.Run(context.Background(), "Hello")

	assert.NoError(t, err)
	assert.Len(t, agent.Conv.Messages, 2)

	userMsg := agent.Conv.Messages[0]
	assert.Equal(t, message.UserRole, userMsg.Role)
	assert.Len(t, userMsg.Content, 1)
	if textBlock, ok := userMsg.Content[0].(message.TextBlock); ok {
		assert.Equal(t, "Hello", textBlock.Text)
	}

	mockLLM.AssertExpectations(t)
}

func TestAgent_Run_WithToolUse(t *testing.T) {
	agent, mockLLM := createTestAgent()

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})
	toolUseMsg := &message.Message{
		Role: message.AssistantRole,
		Content: []message.ContentBlock{
			message.NewToolUseBlock("tool-123", "test_tool", toolInput),
		},
		CreatedAt: time.Now(),
	}

	finalMsg := createTestMessage(message.AssistantRole, "Tool executed successfully")

	mockLLM.On("Generate", mock.Anything, mock.Anything).Return(toolUseMsg, nil).Once()
	mockLLM.On("Generate", mock.Anything, mock.Anything).Return(finalMsg, nil).Once()
	mockLLM.On("CountTokens", mock.Anything, mock.Anything).Return(1, nil).Once()

	err := agent.Run(context.Background(), "Use the test tool")

	assert.NoError(t, err)
	assert.Greater(t, len(agent.Conv.Messages), 2)

	mockLLM.AssertExpectations(t)
}

func TestAgent_Run_LLMError(t *testing.T) {
	agent, mockLLM := createTestAgent()

	expectedError := errors.New("LLM inference failed")

	mockLLM.On("Generate", mock.Anything, mock.Anything).Return(nil, expectedError)

	err := agent.Run(context.Background(), "Hello")

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockLLM.AssertExpectations(t)
}

func TestAgent_executeLocalTool_Success(t *testing.T) {
	agent, _ := createTestAgent()

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})

	result := agent.executeLocalTool("tool-123", "test_tool", toolInput)

	assert.IsType(t, message.ToolResultBlock{}, result)
	toolResult := result.(message.ToolResultBlock)
	assert.Equal(t, "tool-123", toolResult.ToolUseID)
	assert.Equal(t, "test_tool", toolResult.ToolName)
	assert.Equal(t, "test result", toolResult.Content)
	assert.False(t, toolResult.IsError)
}

func TestAgent_executeLocalTool_ToolNotFound(t *testing.T) {
	agent, _ := createTestAgent()

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})

	result := agent.executeLocalTool("tool-123", "nonexistent_tool", toolInput)

	assert.IsType(t, message.ToolResultBlock{}, result)
	toolResult := result.(message.ToolResultBlock)
	assert.Equal(t, "tool-123", toolResult.ToolUseID)
	assert.Equal(t, "nonexistent_tool", toolResult.ToolName)
	assert.Equal(t, "tool not found", toolResult.Content)
	assert.True(t, toolResult.IsError)
}

func TestAgent_executeLocalTool_ToolError(t *testing.T) {
	agent, _ := createTestAgent()

	// Add a tool that returns an error
	errorTool := &tools.ToolDefinition{
		Name:        "error_tool",
		Description: "A tool that errors",
		Function: func(input tools.ToolInput) (string, error) {
			return "", errors.New("tool execution failed")
		},
	}
	agent.ToolBox.Tools = append(agent.ToolBox.Tools, errorTool)

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})

	result := agent.executeLocalTool("tool-123", "error_tool", toolInput)

	assert.IsType(t, message.ToolResultBlock{}, result)
	toolResult := result.(message.ToolResultBlock)
	assert.Equal(t, "tool-123", toolResult.ToolUseID)
	assert.Equal(t, "error_tool", toolResult.ToolName)
	assert.Equal(t, "tool execution failed", toolResult.Content)
	assert.True(t, toolResult.IsError)
}

func TestAgent_executeTool_LocalTool(t *testing.T) {
	agent, _ := createTestAgent()

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})

	result := agent.executeTool(context.Background(), "tool-123", "test_tool", toolInput)

	assert.IsType(t, message.ToolResultBlock{}, result)
	toolResult := result.(message.ToolResultBlock)
	assert.False(t, toolResult.IsError)
	assert.Equal(t, "test result", toolResult.Content)
}
