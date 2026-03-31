package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/honganh1206/tinker/message"
	"github.com/honganh1206/tinker/server/data"
	"github.com/stretchr/testify/assert"
)

func TestSessionResultJSON(t *testing.T) {
	r := &SessionResult{
		SessionID:      "test-123",
		ConversationID: "test-123",
		Status:         StatusSuccess,
		StartedAt:      time.Now(),
		CompletedAt:    time.Now(),
		DurationMs:     1500,
		TokensUsed:     4200,
		RetryCount:     0,
		FinalMessage:   "Done",
		Model:          "claude-4-sonnet",
		Provider:       "Claude",
	}

	bytes, err := json.Marshal(r)
	assert.NoError(t, err)

	var decoded SessionResult
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, StatusSuccess, decoded.Status)
	assert.Equal(t, "test-123", decoded.SessionID)
	assert.Equal(t, 4200, decoded.TokensUsed)
}

func TestExtractFinalMessage(t *testing.T) {
	conv := &data.Conversation{
		ID: "test",
		Messages: []*message.Message{
			{
				Role:    message.UserRole,
				Content: []message.ContentBlock{message.NewTextBlock("hello")},
			},
			{
				Role:    message.AssistantRole,
				Content: []message.ContentBlock{message.NewTextBlock("I can help with that")},
			},
		},
	}

	result := extractFinalMessage(conv)
	assert.Equal(t, "I can help with that", result)
}

func TestExtractFinalMessage_Empty(t *testing.T) {
	conv := &data.Conversation{
		ID:       "test",
		Messages: []*message.Message{},
	}

	result := extractFinalMessage(conv)
	assert.Equal(t, "", result)
}
