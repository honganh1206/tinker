package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/honganh1206/tinker/internal/mcp"
	"github.com/honganh1206/tinker/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupServer builds a server backed by temp dirs for sessions and MCP configs.
func setupServer(t *testing.T) (*Server, string) {
	t.Helper()
	sessionsDir := t.TempDir()
	mcpDir := t.TempDir()
	s := NewServer(nil, nil, sessionsDir, mcpDir)
	return s, sessionsDir
}

// setupWSServer wraps setupServer with an httptest.Server so tests can dial real WebSocket connections.
func setupWSServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, _ := setupServer(t)
	httpServer := httptest.NewServer(s.mux)
	t.Cleanup(httpServer.Close)
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws/stream"
	return s, wsURL
}

// waitForClientCount polls until the server has n connected WS clients.
func waitForClientCount(t *testing.T, s *Server, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return s.clientCount() == n
	}, time.Second, 10*time.Millisecond, "expected %d clients", n)
}

func TestHealthz(t *testing.T) {
	s, _ := setupServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestListSessions_Empty(t *testing.T) {
	s, _ := setupServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var sessions []*storage.Session
	err := json.NewDecoder(w.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

func TestListSessions_WithData(t *testing.T) {
	s, sessionsDir := setupServer(t)

	db, err := storage.NewSession(sessionsDir, "thread-1")
	require.NoError(t, err)
	_, err = storage.CreateContext(db, "ctx-1")
	require.NoError(t, err)
	db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var sessions []*storage.Session
	err = json.NewDecoder(w.Body).Decode(&sessions)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, "thread-1", sessions[0].ID)
}

func TestGetSession(t *testing.T) {
	s, sessionsDir := setupServer(t)

	db, err := storage.NewSession(sessionsDir, "thread-1")
	require.NoError(t, err)
	_, err = storage.CreateContext(db, "ctx-1")
	require.NoError(t, err)
	db.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/thread-1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var session storage.Session
	err = json.NewDecoder(w.Body).Decode(&session)
	require.NoError(t, err)
	assert.Equal(t, "thread-1", session.ID)
	assert.Len(t, session.Contexts, 1)
}

func TestGetSession_NotFound(t *testing.T) {
	s, _ := setupServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteSession(t *testing.T) {
	s, sessionsDir := setupServer(t)

	db, err := storage.NewSession(sessionsDir, "thread-1")
	require.NoError(t, err)
	db.Close()

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/thread-1", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify it's gone
	_, err = storage.GetSession(sessionsDir, "thread-1")
	assert.Error(t, err)
	// Verify .db file is removed
	assert.NoFileExists(t, filepath.Join(sessionsDir, "thread-1.db"))
}

func TestDeleteSession_NotFound(t *testing.T) {
	s, _ := setupServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListMCPConfigs_Empty(t *testing.T) {
	s, _ := setupServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/mcp/configs", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var configs []mcp.ServerConfig
	err := json.NewDecoder(w.Body).Decode(&configs)
	require.NoError(t, err)
	assert.Empty(t, configs)
}

func TestListMCPConfigs_WithData(t *testing.T) {
	s, _ := setupServer(t)

	err := s.mcpStore.Save(mcp.ServerConfig{ID: "test-server", Command: "echo hello"})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/mcp/configs", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var configs []mcp.ServerConfig
	err = json.NewDecoder(w.Body).Decode(&configs)
	require.NoError(t, err)
	assert.Len(t, configs, 1)
	assert.Equal(t, "test-server", configs[0].ID)
}

func TestMethodNotAllowed(t *testing.T) {
	s, _ := setupServer(t)

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/healthz"},
		{http.MethodPost, "/api/sessions"},
		{http.MethodPost, "/api/mcp/configs"},
		{http.MethodPut, "/api/sessions/some-id"},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			s.mux.ServeHTTP(w, req)
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}
