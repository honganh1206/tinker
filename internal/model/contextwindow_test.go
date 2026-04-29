package model

import (
	"context"
	"testing"

	"github.com/honganh1206/tinker/storage"
	"github.com/stretchr/testify/assert"
)

func TestNewContextWindowAndClose(t *testing.T) {
	db, err := storage.NewSession(":memory:", "")
	assert.NoError(t, err)

	cw, err := NewContextWindow(db, &dummyModel{}, "mock")
	assert.NoError(t, err)
	assert.NotNil(t, cw.db)

	// Test record insertion before closing
	err = cw.AddPrompt("test prompt")
	assert.NoError(t, err)

	// Close and test error
	err = cw.Close()
	assert.NoError(t, err)

	// Test adding a prompt after closing
	err = cw.AddPrompt("should fail")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sql: database is closed")
}

func TestContextContinuation(t *testing.T) {
	db, err := storage.NewSession(":memory:", "")
	assert.NoError(t, err)
	defer db.Close()

	model := &MockModel{}

	// Create initial context and add some messages
	cw1, err := NewContextWindow(db, model, "continuation-test")
	assert.NoError(t, err)

	err = cw1.AddPrompt("First message")
	assert.NoError(t, err)

	err = cw1.AddPrompt("Second message")
	assert.NoError(t, err)

	// "Close" the context by creating a new instance
	cw2, err := NewContextWindow(db, model, "continuation-test")
	assert.NoError(t, err)

	// Verify it loads the existing context
	// assert.Equal(t, "continuation-test", cw2.GetCurrentContext())

	// Add a new message and verify all history is available
	err = cw2.AddPrompt("Third message")
	assert.NoError(t, err)

	// Get context ID to check records
	contextID, err := storage.GetContextIDByName(db, "continuation-test")
	assert.NoError(t, err)

	// Verify all messages are present
	records, err := storage.ListLiveRecords(db, contextID)
	assert.NoError(t, err)

	prompts := []string{}
	for _, record := range records {
		if record.Source == storage.Prompt {
			prompts = append(prompts, record.Content)
		}
	}

	assert.Contains(t, prompts, "First message")
	assert.Contains(t, prompts, "Second message")
	assert.Contains(t, prompts, "Third message")
	assert.Equal(t, 3, len(prompts))
}

func TestCallModelInsertRecordError(t *testing.T) {
	db, err := storage.NewSession(":memory:", "")
	assert.NoError(t, err)
	m := &dummyModel{closeDB: true}
	cw, err := NewContextWindow(db, m, "mock")
	assert.NoError(t, err)
	m.cw = cw
	m.events = []storage.Record{{
		Source:    storage.ModelResp,
		Content:   "x",
		Live:      true,
		EstTokens: storage.TokenCount("x"),
	}}
	_, err = cw.CallModel(context.Background())
	assert.Contains(t, err.Error(), "sql: database is closed")
}

func TestCreateAndListContexts(t *testing.T) {
	db, err := storage.NewSession(":memory:", "")
	assert.NoError(t, err)

	cw, err := NewContextWindow(db, &dummyModel{}, "")
	assert.NoError(t, err)
	defer cw.Close()

	contexts, err := cw.ListContexts()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(contexts))
	assert.NotEmpty(t, contexts[0].Name)

	err = cw.CreateContext("test-context")
	assert.NoError(t, err)

	contexts, err = cw.ListContexts()
	assert.NoError(t, err)
	assert.Equal(t, 2, len(contexts))

	found := false
	for _, ctx := range contexts {
		if ctx.Name == "test-context" {
			found = true
			break
		}
	}
	assert.True(t, found)
}
