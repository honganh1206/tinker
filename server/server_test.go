package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/honganh1206/tinker/session"
	"github.com/honganh1206/tinker/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*Server, *store.FileStore) {
	t.Helper()
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	require.NoError(t, err)
	srv := New(fs, nil)
	return srv, fs
}

func seedSession(t *testing.T, fs *store.FileStore, id string, status session.Status) {
	t.Helper()
	err := fs.Save(&session.SessionResult{
		SessionID:    id,
		Status:       status,
		Prompt:       "test prompt for " + id,
		StartedAt:    time.Now(),
		CompletedAt:  time.Now(),
		DurationMs:   1500,
		TokensUsed:   4200,
		FinalMessage: "done",
		Model:        "claude-4-sonnet",
		Provider:     "anthropic",
	})
	require.NoError(t, err)
}

func TestListSessions(t *testing.T) {
	srv, fs := setupTestServer(t)
	seedSession(t, fs, "sess-1", session.StatusSuccess)
	seedSession(t, fs, "sess-2", session.StatusFailed)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var summaries []store.SessionSummary
	err := json.Unmarshal(w.Body.Bytes(), &summaries)
	require.NoError(t, err)
	assert.Len(t, summaries, 2)
}

func TestGetSession(t *testing.T) {
	srv, fs := setupTestServer(t)
	seedSession(t, fs, "sess-1", session.StatusSuccess)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/sess-1", nil)
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result session.SessionResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "sess-1", result.SessionID)
}

func TestGetSession_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHealthz(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}
