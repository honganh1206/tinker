package store

import "github.com/honganh1206/tinker/session"

type SessionSummary struct {
	ID         string         `json:"id"`
	Status     session.Status `json:"status"`
	Prompt     string         `json:"prompt"`
	Model      string         `json:"model"`
	StartedAt  string         `json:"started_at"`
	DurationMs int64          `json:"duration_ms"`
}

type Store interface {
	Save(result *session.SessionResult) error
	Load(id string) (*session.SessionResult, error)
	List() ([]SessionSummary, error)
}
