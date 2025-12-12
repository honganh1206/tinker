package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/honganh1206/tinker/mcp"
	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/server"
	"github.com/honganh1206/tinker/server/data"
	"github.com/honganh1206/tinker/server/mocks"
	"github.com/honganh1206/tinker/tools"
	"github.com/honganh1206/tinker/ui"
)

// Test helpers
func createTestAgent() (*Agent, *mocks.MockLLMClient, *mocks.MockAPIClient) {
	mockLLM := &mocks.MockLLMClient{}
	mockAPI := &mocks.MockAPIClient{}

	conv, _ := data.NewConversation()
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

	ctl := ui.NewController()
	agent := New(&Config{
		LLM:          mockLLM,
		Conversation: conv,
		ToolBox:      toolBox,
		Client:       mockAPI,
		MCPConfigs:   []mcp.ServerConfig{},
		Streaming:    false,
		Controller:   ctl,
	})
	return agent, mockLLM, mockAPI
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
		name      string
		streaming bool
		mcpCount  int
	}{
		{
			name:      "creates agent with streaming enabled",
			streaming: true,
			mcpCount:  2,
		},
		{
			name:      "creates agent with streaming disabled",
			streaming: false,
			mcpCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLLM := &mocks.MockLLMClient{}
			conv, _ := data.NewConversation()
			toolBox := &tools.ToolBox{Tools: []*tools.ToolDefinition{}}

			mcpConfigs := make([]mcp.ServerConfig, tt.mcpCount)
			for i := 0; i < tt.mcpCount; i++ {
				mcpConfigs[i] = mcp.ServerConfig{
					ID:      "test-server",
					Command: "test-command",
				}
			}

			realClient := server.NewClient("")
			agent := New(&Config{
				LLM:          mockLLM,
				Conversation: conv,
				ToolBox:      toolBox,
				Client:       realClient,
				MCPConfigs:   mcpConfigs,
				Streaming:    tt.streaming,
			})

			assert.NotNil(t, agent)
			assert.Equal(t, mockLLM, agent.LLM)
			assert.Equal(t, conv, agent.Conv)
			assert.Equal(t, toolBox, agent.ToolBox)
			assert.NotNil(t, agent.client)
			assert.Equal(t, tt.streaming, agent.streaming)
			assert.Equal(t, tt.mcpCount, len(agent.MCP.ServerConfigs))
			assert.NotNil(t, agent.MCP.ActiveServers)
			assert.NotNil(t, agent.MCP.Tools)
			assert.NotNil(t, agent.MCP.ToolMap)
		})
	}
}

func TestAgent_Run_SimpleTextResponse(t *testing.T) {
	agent, mockLLM, mockClient := createTestAgent()

	// Setup mocks
	mockLLM.On("SummarizeHistory", mock.Anything, 20).Return([]*message.Message{})
	mockLLM.On("ToNativeTools", mock.Anything).Return(nil)
	mockLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(
		createTestMessage(message.AssistantRole, "Hello, how can I help?"), nil)
	mockLLM.On("CountTokens", mock.MatchedBy(func(ctx context.Context) bool { return true })).Return(0, nil).Once()

	mockClient.On("SaveConversation", mock.Anything).Return(nil)
	mockClient.On("UpdateTokenCount", mock.Anything, mock.Anything).Return(nil)

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
	mockClient.AssertExpectations(t)
}

func TestAgent_Run_WithToolUse(t *testing.T) {
	agent, mockLLM, mockClient := createTestAgent()

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
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(toolUseMsg, nil).Once()
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(finalMsg, nil).Once()
	mockLLM.On("CountTokens", mock.MatchedBy(func(ctx context.Context) bool { return true })).Return(1, nil).Once()

	mockClient.On("SaveConversation", mock.Anything).Return(nil)
	mockClient.On("UpdateTokenCount", mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	userInput := "Use the test tool"
	onDelta := func(delta string) {}

	err := agent.Run(ctx, userInput, onDelta)

	assert.NoError(t, err)
	assert.Greater(t, len(agent.Conv.Messages), 2) // Should have multiple messages

	mockLLM.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestAgent_Run_LLMError(t *testing.T) {
	agent, mockLLM, _ := createTestAgent()

	expectedError := errors.New("LLM inference failed")

	// Setup mocks
	mockLLM.On("SummarizeHistory", mock.Anything, 20).Return([]*message.Message{})
	mockLLM.On("ToNativeTools", mock.Anything).Return(nil)
	mockLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(nil, expectedError)

	ctx := context.Background()
	userInput := "Hello"
	onDelta := func(delta string) {}

	err := agent.Run(ctx, userInput, onDelta)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	mockLLM.AssertExpectations(t)
}

func TestAgent_executeLocalTool_Success(t *testing.T) {
	agent, _, _ := createTestAgent()

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
	agent, _, _ := createTestAgent()

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
	agent, _, _ := createTestAgent()

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

func TestAgent_runSubagent_Success(t *testing.T) {
	agent, _, _ := createTestAgent()

	// Create a real subagent with mocked LLM
	subLLM := &mocks.MockLLMClient{}
	subToolBox := &tools.ToolBox{Tools: []*tools.ToolDefinition{}}
	subLLM.On("ToNativeTools", subToolBox.Tools).Return(nil)
	realSubagent := NewSubagent(&Config{
		LLM:       subLLM,
		ToolBox:   subToolBox,
		Streaming: false,
	})
	agent.Sub = realSubagent

	expectedResponse := &message.Message{
		Role: message.AssistantRole,
		Content: []message.ContentBlock{
			message.NewTextBlock("Subagent completed task"),
		},
	}

	subLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	subLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(expectedResponse, nil)
	subLLM.On("ToNativeMessage", expectedResponse).Return(nil)

	toolInput, _ := json.Marshal(map[string]string{"query": "test query"})

	result, err := agent.runSubagent("tool-123", "tool_name", "tool description", toolInput)

	assert.NoError(t, err)
	assert.Equal(t, expectedResponse, result)

	subLLM.AssertExpectations(t)
}

func TestAgent_runSubagent_InvalidJSON(t *testing.T) {
	agent, _, _ := createTestAgent()

	invalidJSON := []byte(`{"invalid": json}`)

	result, err := agent.runSubagent("tool-123", "tool_name", "tool description", invalidJSON)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestAgent_runSubagent_SubagentError(t *testing.T) {
	agent, _, _ := createTestAgent()

	subLLM := &mocks.MockLLMClient{}
	subToolBox := &tools.ToolBox{Tools: []*tools.ToolDefinition{}}
	subLLM.On("ToNativeTools", subToolBox.Tools).Return(nil)
	subagent := NewSubagent(&Config{
		LLM:       subLLM,
		ToolBox:   subToolBox,
		Streaming: false,
	})
	agent.Sub = subagent

	expectedError := errors.New("subagent execution failed")
	subLLM.On("ToNativeMessage", mock.Anything).Return(nil)
	subLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(nil, expectedError)

	toolInput, _ := json.Marshal(map[string]string{"query": "test query"})

	result, err := agent.runSubagent("tool-123", "tool_name", "tool description", toolInput)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "inference failed")
	assert.Nil(t, result)

	subLLM.AssertExpectations(t)
}

func TestAgent_streamResponse_Success(t *testing.T) {
	agent, mockLLM, _ := createTestAgent()

	expectedMessage := createTestMessage(message.AssistantRole, "Streamed response")
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(expectedMessage, nil)

	ctx := context.Background()
	onDelta := func(delta string) {}

	result, err := agent.streamResponse(ctx, onDelta)

	assert.NoError(t, err)
	assert.Equal(t, expectedMessage, result)
	mockLLM.AssertExpectations(t)
}

func TestAgent_streamResponse_Error(t *testing.T) {
	agent, mockLLM, _ := createTestAgent()

	expectedError := errors.New("streaming failed")
	mockLLM.On("RunInference", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.Anything, false).Return(nil, expectedError)

	ctx := context.Background()
	onDelta := func(delta string) {}

	result, err := agent.streamResponse(ctx, onDelta)

	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Nil(t, result)
	mockLLM.AssertExpectations(t)
}

func TestAgent_executeTool_LocalTool(t *testing.T) {
	agent, _, _ := createTestAgent()

	toolInput, _ := json.Marshal(map[string]string{"query": "test"})
	deltaReceived := ""
	onDelta := func(delta string) {
		deltaReceived += delta
	}

	result := agent.executeTool("tool-123", "test_tool", toolInput, onDelta)

	assert.IsType(t, message.ToolResultBlock{}, result)
	toolResult := result.(message.ToolResultBlock)
	assert.False(t, toolResult.IsError)
	assert.Contains(t, deltaReceived, "test_tool") // Should contain success message
}
