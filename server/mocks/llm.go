package mocks

import (
	"context"

	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/tools"
	"github.com/stretchr/testify/mock"
)

type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) RunInference(ctx context.Context, onDelta func(string), streaming bool) (*message.Message, error) {
	args := m.Called(ctx, onDelta, streaming)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*message.Message), args.Error(1)
}

func (m *MockLLMClient) SummarizeHistory(history []*message.Message, threshold int) []*message.Message {
	args := m.Called(history, threshold)
	return args.Get(0).([]*message.Message)
}

func (m *MockLLMClient) TruncateMessage(msg *message.Message, threshold int) *message.Message {
	args := m.Called(msg, threshold)
	return args.Get(0).(*message.Message)
}

func (m *MockLLMClient) ProviderName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockLLMClient) ToNativeHistory(history []*message.Message) error {
	args := m.Called(history)
	return args.Error(0)
}

func (m *MockLLMClient) ToNativeMessage(msg *message.Message) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockLLMClient) ToNativeTools(tools []*tools.ToolDefinition) error {
	args := m.Called(tools)
	return args.Error(0)
}

func (m *MockLLMClient) CountTokens(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func (m *MockLLMClient) ModelName() string {
	args := m.Called()
	return args.String(0)
}
