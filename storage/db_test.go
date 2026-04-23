package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSession_CreatesNewDB(t *testing.T) {
	dir := t.TempDir()
	threadID := "1234567890"

	db, err := OpenSession(dir, threadID)
	if err != nil {
		t.Fatalf("open session: %v", err)
	}
	defer db.Close()

	// File should exist
	dbPath := filepath.Join(dir, threadID+".db")
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
	threadID := "1234567890"

	// Create and insert data
	db1, err := OpenSession(dir, threadID)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_, err = CreateContext(db1, "test-context")
	if err != nil {
		t.Fatalf("create context: %v", err)
	}
	db1.Close()

	// Reopen and verify data persists
	db2, err := OpenSession(dir, threadID)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer db2.Close()

	_, err = GetContextByName(db2, "test-context")
	if err != nil {
		t.Fatalf("context not found after reopen: %v", err)
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()

	// Create a couple of session dbs
	db1, _ := OpenSession(dir, "aaa")
	db1.Close()
	db2, _ := OpenSession(dir, "bbb")
	db2.Close()

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatalf("ListSessionDBs: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}
