package mocks

import (
	"github.com/honganh1206/tinker/server/data"
	"github.com/stretchr/testify/mock"
)

type MockAPIClient struct {
	mock.Mock
}

func (m *MockAPIClient) SaveConversation(conv *data.Conversation) error {
	args := m.Called(conv)
	return args.Error(0)
}

func (m *MockAPIClient) UpdateTokenCount(conversationID string, tokenCount int) error {
	args := m.Called(conversationID, tokenCount)
	return args.Error(0)
}

func (m *MockAPIClient) GetPlan(id string) (*data.Plan, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*data.Plan), args.Error(1)
}

func (m *MockAPIClient) CreatePlan(conversationID string) (*data.Plan, error) {
	args := m.Called(conversationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*data.Plan), args.Error(1)
}

func (m *MockAPIClient) SavePlan(p *data.Plan) error {
	args := m.Called(p)
	return args.Error(0)
}

func (m *MockAPIClient) CreateConversation() (*data.Conversation, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*data.Conversation), args.Error(1)
}

func (m *MockAPIClient) ListConversations() ([]data.ConversationMetadata, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]data.ConversationMetadata), args.Error(1)
}

func (m *MockAPIClient) GetConversation(id string) (*data.Conversation, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*data.Conversation), args.Error(1)
}

func (m *MockAPIClient) GetLatestConversationID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockAPIClient) ListPlans() ([]data.PlanInfo, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]data.PlanInfo), args.Error(1)
}

func (m *MockAPIClient) DeletePlan(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAPIClient) DeletePlans(ids []string) (map[string]error, error) {
	args := m.Called(ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]error), args.Error(1)
}
