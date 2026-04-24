package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	Prompt RecordType = iota
	ModelResp
	ToolUse
	ToolResult
	SystemPrompt
)

// CreateContext creates a new context
func CreateContext(db *sql.DB, name string) (Context, error) {
	if name == "" {
		return Context{}, fmt.Errorf("context name cannot be empty")
	}

	// Check existing context
	_, err := GetContextByName(db, name)
	if err == nil {
		return Context{}, fmt.Errorf("context %q already exists", name)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Context{}, fmt.Errorf("check existing context: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	_, err = db.Exec(
		`INSERT INTO contexts (id, name, start_time) VALUES (?, ?, ?)`, id, name, now,
	)
	if err != nil {
		return Context{}, fmt.Errorf("create context: %w", err)
	}

	return Context{
		Name:      name,
		StartTime: now,
		ID:        id,
	}, nil
}

func InsertRecord(
	db *sql.DB,
	contextID string,
	source RecordType,
	content string,
	live bool,
) (Record, error) {
	now := time.Now().UTC()
	t := TokenCount(content)
	res, err := db.Exec(
		`INSERT INTO records (context_id, ts, source, content, live, est_tokens) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
		contextID, now, int(source), content, live, t,
	)
	if err != nil {
		return Record{}, fmt.Errorf("insert record: %w", err)
	}

	// Track the latest inserted record
	id, err := res.LastInsertId()
	if err != nil {
		return Record{}, fmt.Errorf("get last insert id: %w", err)
	}
	return Record{
		ID:        id,
		Timestamp: now,
		Source:    source,
		Content:   content,
		Live:      live,
		EstTokens: t,
		ContextID: contextID,
	}, nil
}

func InsertRecordTx(
	tx *sql.Tx,
	contextID string,
	source RecordType,
	content string,
	live bool,
) (Record, error) {
	now := time.Now().UTC()
	t := TokenCount(content)
	res, err := tx.Exec(
		`INSERT INTO records (context_id, ts, source, content, live, est_tokens) 
		 VALUES (?, ?, ?, ?, ?, ?)`,
		contextID, now, int(source), content, live, t,
	)
	if err != nil {
		return Record{}, fmt.Errorf("insert record tx: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Record{}, fmt.Errorf("get last insert id tx: %w", err)
	}
	return Record{
		ID:        id,
		Timestamp: now,
		Source:    source,
		Content:   content,
		Live:      live,
		EstTokens: t,
		ContextID: contextID,
	}, nil
}

func GetContext(db *sql.DB, contextID string) (Context, error) {
	var c Context
	err := db.QueryRow(
		`SELECT id, name, start_time
		 FROM contexts WHERE id = ?`,
		contextID,
	).Scan(&c.ID, &c.Name, &c.StartTime)
	if err != nil {
		return Context{}, fmt.Errorf("get context %s: %w", contextID, err)
	}
	return c, nil
}

// GetContextIDByName gets the internal UUID by context name.
func GetContextIDByName(db *sql.DB, name string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM contexts WHERE name = ?`, name).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("get context ID for '%s': %w", name, err)
	}
	return id, nil
}

// GetContextByName retrieves a context by name.
func GetContextByName(db *sql.DB, name string) (Context, error) {
	var c Context
	err := db.QueryRow(
		`SELECT id, name, start_time
		 FROM contexts WHERE name = ?`,
		name,
	).Scan(&c.ID, &c.Name, &c.StartTime)
	if err != nil {
		return Context{}, fmt.Errorf("get context '%s': %w", name, err)
	}
	return c, nil
}

// AddContextTool adds a tool name to a specific context
func AddContextTool(db *sql.DB, contextID, toolName string) (ContextTool, error) {
	now := time.Now().UTC()
	res, err := db.Exec(
		`INSERT INTO context_tools (context_id, tool_name, created_at)
		 VALUES (?, ?, ?)`,
		contextID, toolName, now,
	)
	if err != nil {
		return ContextTool{}, fmt.Errorf("add context tool: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return ContextTool{}, fmt.Errorf("get last insert id: %w", err)
	}
	return ContextTool{
		ID:        id,
		ContextID: contextID,
		ToolName:  toolName,
		CreatedAt: now,
	}, nil
}

// HasContextTool checks if a specific tool is available in a context.
func HasContextTool(db *sql.DB, contextID, toolName string) (bool, error) {
	var exists bool
	err := db.QueryRow(
		`SELECT 1 FROM context_tools WHERE context_id = ? AND tool_name = ?`,
		contextID, toolName,
	).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("check context tool: %w", err)
	}
	return exists, nil
}


// ListLiveRecords returns all live records in a context in a timestamp order
func ListLiveRecords(db *sql.DB, contextID string) ([]Record, error) {
	return listRecordsWhere(db, "context_id = ? AND live = 1", contextID)
}

func listRecordsWhere(db *sql.DB, whereClause string, args ...any) ([]Record, error) {
	query := fmt.Sprintf(`
		SELECT id, context_id, ts, source, content, live, est_tokens 
		 FROM records WHERE %s ORDER BY ts ASC
		`, whereClause,
	)
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()

	var recs []Record
	for rows.Next() {
		var r Record
		var src int
		if err := rows.Scan(
			&r.ID,
			&r.ContextID,
			&r.Timestamp,
			&src,
			&r.Content,
			&r.Live,
			&r.EstTokens,
		); err != nil {
			return nil, fmt.Errorf("scan record: %w", err)
		}
		// Type assertion?
		r.Source = RecordType(src)
		recs = append(recs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("records rows: %w", err)
	}
	return recs, nil
}

// ListContexts returns all contexts ordered by start time.
func ListContexts(db *sql.DB) ([]Context, error) {
	rows, err := db.Query(
		`SELECT id, name, start_time
		 FROM contexts ORDER BY start_time DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query contexts: %w", err)
	}
	defer rows.Close()

	var contexts []Context
	for rows.Next() {
		var c Context
		if err := rows.Scan(&c.ID, &c.Name, &c.StartTime); err != nil {
			return nil, fmt.Errorf("scan context: %w", err)
		}
		contexts = append(contexts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("contexts rows: %w", err)
	}
	return contexts, nil
}

// DeleteContextByName removes a context and all its records by name.
func DeleteContextByName(db *sql.DB, name string) error {
	ctx, err := GetContextByName(db, name)
	if err != nil {
		return err
	}
	return DeleteContext(db, ctx.ID)
}

// DeleteContext removes a context and all its records by ID.
func DeleteContext(db *sql.DB, contextID string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM records WHERE context_id = ?`, contextID)
	if err != nil {
		return fmt.Errorf("delete context records: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM contexts WHERE id = ?`, contextID)
	if err != nil {
		return fmt.Errorf("delete context: %w", err)
	}

	return tx.Commit()
}
