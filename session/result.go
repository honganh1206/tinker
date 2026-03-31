package session

import "time"

type Status string

const (
	StatusSuccess Status = "success"
	StatusPartial Status = "partial"
	StatusFailed  Status = "failed"
)

type SessionResult struct {
	SessionID      string    `json:"session_id"`
	ConversationID string    `json:"conversation_id"`
	Status         Status    `json:"status"`
	StartedAt      time.Time `json:"started_at"`
	CompletedAt    time.Time `json:"completed_at"`
	DurationMs     int64     `json:"duration_ms"`
	TokensUsed     int       `json:"tokens_used"`
	RetryCount     int       `json:"retry_count"`
	FinalMessage   string    `json:"final_message"`
	Error          string    `json:"error,omitempty"`
	Model          string    `json:"model"`
	Provider       string    `json:"provider"`
}
