package store

import (
	"os"
	"testing"
	"time"

	"github.com/honganh1206/tinker/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestResult(id string) *session.SessionResult {
	return &session.SessionResult{
		SessionID:    id,
		Status:       session.StatusSuccess,
		Prompt:       "test prompt",
		StartedAt:    time.Now(),
		CompletedAt:  time.Now(),
		DurationMs:   1500,
		TokensUsed:   4200,
		FinalMessage: "Done",
		Model:        "claude-4-sonnet",
		Provider:     "Claude",
	}
}

func TestFileStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	require.NoError(t, err)

	result := newTestResult("test-123")
	err = fs.Save(result)
	require.NoError(t, err)

	loaded, err := fs.Load("test-123")
	require.NoError(t, err)
	assert.Equal(t, "test-123", loaded.SessionID)
	assert.Equal(t, session.StatusSuccess, loaded.Status)
	assert.Equal(t, "test prompt", loaded.Prompt)
	assert.Equal(t, 4200, loaded.TokensUsed)
}

func TestFileStore_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	require.NoError(t, err)

	_, err = fs.Load("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	require.NoError(t, err)

	err = fs.Save(newTestResult("session-1"))
	require.NoError(t, err)
	err = fs.Save(newTestResult("session-2"))
	require.NoError(t, err)

	summaries, err := fs.List()
	require.NoError(t, err)
	assert.Len(t, summaries, 2)
}

func TestFileStore_List_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	require.NoError(t, err)

	summaries, err := fs.List()
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func TestFileStore_DefaultDir(t *testing.T) {
	fs, err := NewFileStore("")
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Contains(t, fs.dir, home)
	assert.Contains(t, fs.dir, ".tinker")
	assert.Contains(t, fs.dir, "sessions")
}
