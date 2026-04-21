package storage

import "time"

// RecordType distinguishes entry kinds
type RecordType int

// Record is one row in context history
type Record struct {
	ID        int64      `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	Source    RecordType `json:"source"`
	Content   string     `json:"content"`
	// Flag to control whether record is sent to LLM or not
	Live bool `json:"live"`
	// Estimated number of tokens, counted by built-in tokenizer
	EstTokens int    `json:"est_tokens"`
	ContextID string `json:"context_id"`
}

// Context represents a named context window with metadata
type Context struct {
	ID                     string    `json:"id"`
	Name                   string    `json:"name"`
	StartTime              time.Time `json:"start_time"`
}

// ContextTool represents a tool available in a specific context
type ContextTool struct {
	ID        int64     `json:"id"`
	ContextID string    `json:"context_id"`
	ToolName  string    `json:"tool_name"`
	CreatedAt time.Time `json:"created_at"`
}
