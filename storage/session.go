package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// OpenSession opens a database (a session) to store context window in.
// NOTE: LLM conversations are stored in SQLite.
// If you don't care about persistent storage for your context,
// just specify ":memory:" as your database path,
// which creates a SQLite database in RAM instead of disk.
func OpenSession(dir, id string) (*sql.DB, error) {
	var dbPath string
	if dir == ":memory:" {
		dbPath = ":memory:"
	} else {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create sessions dir: %w", err)
		}
		dbPath = filepath.Join(dir, id+".db")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	return db, nil
}

// NewSession creates a database/session to store context window in.
// We call this before creating context windows.
func NewSession(sessionsDir, threadID string) (*sql.DB, error) {
	var dbPath string
	if sessionsDir == ":memory:" {
		dbPath = ":memory:"
	} else {
		if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
			return nil, fmt.Errorf("create sessions dir: %w", err)
		}
		dbPath = filepath.Join(sessionsDir, threadID+".db")
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open session: %w", err)
	}

	if err = initializeSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return db, nil
}

// initializeSchema ensures the contexts and records tables and indexes exist
func initializeSchema(db *sql.DB) error {
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

func ListSessions(dir string) ([]*Session, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session directory does not exist: %w", err)
		}
		return nil, fmt.Errorf("failed to read session directory: %w", err)
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".db")
		dbPath := filepath.Join(dir, entry.Name())

		ss, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			continue
		}

		ctxs, err := ListContexts(ss)
		if err != nil {
			ss.Close()
			continue
		}

		session := &Session{
			ID:           id,
			ContextCount: len(ctxs),
		}
		if len(ctxs) > 0 {
			session.Name = ctxs[0].Name
			session.StartTime = ctxs[0].StartTime.Format("2006-01-02T15:04:05Z")
		}

		ss.Close()
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func GetSession(dir, id string) (*Session, error) {
	dbPath := filepath.Join(dir, id+".db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("session %s not found: %w", id, err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	defer db.Close()
	contexts, err := ListContexts(db)
	if err != nil {
		return nil, fmt.Errorf("failed to list contexts: %w", err)
	}

	var allRecords []Record
	for _, ctx := range contexts {
		records, err := ListLiveRecords(db, ctx.ID)
		if err != nil {
			continue
		}
		allRecords = append(allRecords, records...)
	}

	return &Session{
		ID:       id,
		Contexts: contexts,
		Records:  allRecords,
	}, nil
}

func DeleteSession(dir, id string) error {
	dbPath := filepath.Join(dir, id+".db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("session %s not found: %w", id, err)
	}

	if err := os.Remove(dbPath); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}
