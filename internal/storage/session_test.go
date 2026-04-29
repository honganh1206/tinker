package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpenSession_CreatesNewDB(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"

	db, err := NewSession(dir, id)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer db.Close()

	// File should exist
	dbPath := filepath.Join(dir, id+".db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("expected db file at %s", dbPath)
	}

	// Schema should be initialized (contexts table should exist)
	var name string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='contexts'").Scan(&name)
	if err != nil {
		t.Fatalf("schema not initialized: %v", err)
	}
}

func TestOpenSession_ReopensExistingDB(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"

	// Create and insert data
	db, err := NewSession(dir, id)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_, err = CreateContext(db, "test-context")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}
	db.Close()

	// Reopen and verify data persists
	db, err = OpenSession(dir, id)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer db.Close()

	_, err = GetContextByName(db, "test-context")
	if err != nil {
		t.Fatalf("context not found after reopen: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"
	s, err := NewSession(dir, id)
	require.NoError(t, err)

	seedSession(t, s, "sess-1")

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestListSessions_Empty(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"
	db, err := NewSession(dir, id)
	require.NoError(t, err)
	db.Close()

	sessions, err := ListSessions(dir)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestGetSession(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"
	s, err := NewSession(dir, id)
	require.NoError(t, err)

	seedSession(t, s, "sess-1")

	detail, err := GetSession(dir, id)
	require.NoError(t, err)
	assert.Equal(t, id, detail.ID)
	assert.NotEmpty(t, detail.Contexts)
}

func TestGetSession_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := NewSession(dir, "")
	require.NoError(t, err)

	_, err = GetSession(dir, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteSession(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"
	s, err := NewSession(dir, id)
	require.NoError(t, err)

	seedSession(t, s, id)

	err = DeleteSession(dir, id)
	require.NoError(t, err)

	_, err = GetSession(dir, id)
	assert.Error(t, err)
}

func TestDeleteSession_NotFound(t *testing.T) {
	dir := t.TempDir()
	id := "1234567890"
	_, err := NewSession(dir, id)
	require.NoError(t, err)

	err = DeleteSession(dir, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func seedSession(t *testing.T, db *sql.DB, id string) {
	t.Helper()

	_, err := CreateContext(db, "test-context")
	require.NoError(t, err)

	_, err = InsertRecord(db, "test-context", Prompt, "hello", true)
	require.NoError(t, err)
	require.NoError(t, db.Close())
}
