package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// NewContextDB opens a database to store context window in.
// We call this before creating context windows.
// NOTE: LLM conversations are stored in SQLite.
// If you don't care about persistent storage for your context,
// just specify ":memory:" as your database path,
// which creates a sqlite database in RAM instead of disk.
func NewContextDB(dbpath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbpath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err = InitializeSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}

// InitializeSchema ensures the contexts and records tables and indexes exist
func InitializeSchema(db *sql.DB) error {
	const baseTables = `
		CREATE TABLE IF NOT EXISTS contexts (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			start_time DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS records (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			context_id TEXT NOT NULL,
			ts         DATETIME NOT NULL,
			source     INTEGER NOT NULL,
			content    TEXT NOT NULL,
			live       BOOLEAN NOT NULL,
			est_tokens INTEGER NOT NULL,
			FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS context_tools (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			context_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (context_id) REFERENCES contexts(id) ON DELETE CASCADE,
			UNIQUE(context_id, tool_name)
		);
`

	_, err := db.Exec(baseTables)
	if err != nil {
		return fmt.Errorf("create base tables: %w", err)
	}

	const indexes = `
		CREATE INDEX IF NOT EXISTS idx_context_live ON records(context_id, live);
		CREATE INDEX IF NOT EXISTS idx_context_ts ON records(context_id, ts);
		CREATE INDEX IF NOT EXISTS idx_context_tools_context ON context_tools(context_id);
`
	_, err = db.Exec(indexes)
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	return nil
}

// OpenSession opens (or creates) a per-session SQLite database.
// Each session is identified by a thread ID and stored as <threadID>.db
// in the given directory.
func OpenSession(sessionsDir, threadID string) (*sql.DB, error) {
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}
	dbPath := filepath.Join(sessionsDir, threadID+".db")
	return NewContextDB(dbPath)
}

// ListSession returns the thread IDs of all session databases in the directory.
func ListSessions(sessionsDir string) ([]string, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) == ".db" {
			sessions = append(sessions, strings.TrimSuffix(name, ".db"))
		}
	}
	return sessions, nil
}
