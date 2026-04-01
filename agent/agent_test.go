package agent

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
)

// Mock LLM client for testing
type mockLLMClient struct {
	mock.Mock
}

func (m *mockLLMClient) RunInference(ctx context.Context, onDelta func(string), streaming bool) (*message.Message, error) {
	args := m.Called(ctx, onDelta, streaming)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*message.Message), args.Error(1)
}

func (m *mockLLMClient) SummarizeHistory(history []*message.Message, threshold int) []*message.Message {
	args := m.Called(history, threshold)
	return args.Get(0).([]*message.Message)
}

func (m *mockLLMClient) TruncateMessage(msg *message.Message, threshold int) *message.Message {
	args := m.Called(msg, threshold)
	return args.Get(0).(*message.Message)
}

func (m *mockLLMClient) ProviderName() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockLLMClient) ModelName() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockLLMClient) ToNativeHistory(history []*message.Message) error {
	args := m.Called(history)
	return args.Error(0)
}

func (m *mockLLMClient) ToNativeMessage(msg *message.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *mockLLMClient) ToNativeTools(tools []*tools.ToolDefinition) error {
	args := m.Called(tools)
	return args.Error(0)
}

func (m *mockLLMClient) CountTokens(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
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
		Logger:       slog.Default(),
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
			assert.Equal(t, tt.mcpCount, len(agent.MCP.ServerConfigs))
			assert.NotNil(t, agent.MCP.ActiveServers)
			assert.NotNil(t, agent.MCP.Tools)
			assert.NotNil(t, agent.MCP.ToolMap)
		})
	}
}

func TestAgent_Run_SimpleTextResponse(t *testing.T) {
	agent, mockLLM := createTestAgent()

	// Setup mocks
	mockLLM.On("SummarizeHistory", mock.Anything, 20).Return([]*message.Message{})
	mockLLM.On("ToNativeTools", mock.Anything).Return(nil)
	mockLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, true).Return(
		createTestMessage(message.AssistantRole, "Hello, how can I help?"), nil)
	mockLLM.On("CountTokens", mock.MatchedBy(func(ctx context.Context) bool { return true })).Return(0, nil).Once()

	ctx := context.Background()
	userInput := "Hello"
	deltaReceived := ""
	onDelta := func(delta string) {
		deltaReceived += delta
	}

	err := agent.Run(ctx, userInput, onDelta)

	assert.NoError(t, err)
	assert.Len(t, agent.Conv.Messages, 2) // User message + Assistant message

	// Verify user message was added
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

	// Create tool use message
	toolInput, _ := json.Marshal(map[string]string{"query": "test"})
	toolUseMsg := &message.Message{
		Role: message.AssistantRole,
		Content: []message.ContentBlock{
			message.NewToolUseBlock("tool-123", "test_tool", toolInput),
		},
		CreatedAt: time.Now(),
	}

	// Final response after tool execution
	finalMsg := createTestMessage(message.AssistantRole, "Tool executed successfully")

	// Setup mocks
	mockLLM.On("SummarizeHistory", mock.Anything, 20).Return([]*message.Message{})
	mockLLM.On("ToNativeTools", mock.Anything).Return(nil)
	mockLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, true).Return(toolUseMsg, nil).Once()
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, true).Return(finalMsg, nil).Once()
	mockLLM.On("CountTokens", mock.MatchedBy(func(ctx context.Context) bool { return true })).Return(1, nil).Once()

	ctx := context.Background()
	userInput := "Use the test tool"
	onDelta := func(delta string) {}

	err := agent.Run(ctx, userInput, onDelta)

	assert.NoError(t, err)
	assert.Greater(t, len(agent.Conv.Messages), 2) // Should have multiple messages

	mockLLM.AssertExpectations(t)
}

func TestAgent_Run_LLMError(t *testing.T) {
	agent, mockLLM := createTestAgent()

	expectedError := errors.New("LLM inference failed")

	// Setup mocks
	mockLLM.On("SummarizeHistory", mock.Anything, 20).Return([]*message.Message{})
	mockLLM.On("ToNativeTools", mock.Anything).Return(nil)
	mockLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, true).Return(nil, expectedError)

	ctx := context.Background()
	userInput := "Hello"
	onDelta := func(delta string) {}

	err := agent.Run(ctx, userInput, onDelta)

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

func TestAgent_streamResponse_Success(t *testing.T) {
	agent, mockLLM := createTestAgent()

	expectedMessage := createTestMessage(message.AssistantRole, "Streamed response")
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, true).Return(expectedMessage, nil)

	ctx := context.Background()
	onDelta := func(delta string) {}

	result, err := agent.streamResponse(ctx, onDelta)

	assert.NoError(t, err)
	assert.Equal(t, expectedMessage, result)
	mockLLM.AssertExpectations(t)
}

func TestAgent_streamResponse_Error(t *testing.T) {
	agent, mockLLM := createTestAgent()

	expectedError := errors.New("streaming failed")
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, true).Return(nil, expectedError)

	ctx := context.Background()
	onDelta := func(delta string) {}

	result, err := agent.streamResponse(ctx, onDelta)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Nil(t, result)
	mockLLM.AssertExpectations(t)
}

func TestAgent_executeTool_LocalTool(t *testing.T) {
	agent, _ := createTestAgent()

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})
	onDelta := func(delta string) {}

	result := agent.executeTool("tool-123", "test_tool", toolInput, onDelta)

	assert.IsType(t, message.ToolResultBlock{}, result)
	toolResult := result.(message.ToolResultBlock)
	assert.False(t, toolResult.IsError)
	assert.Equal(t, "test result", toolResult.Content)
}
