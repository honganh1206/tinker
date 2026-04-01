package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/honganh1206/tinker/session"
)

type FileStore struct {
	dir string
}

func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = filepath.Join(home, ".tinker", "sessions")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &FileStore{dir: dir}, nil
}

func (fs *FileStore) Save(result *session.SessionResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	path := filepath.Join(fs.dir, result.SessionID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

func (fs *FileStore) Load(id string) (*session.SessionResult, error) {
	path := filepath.Join(fs.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %s not found", id)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var result session.SessionResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &result, nil
}

func (fs *FileStore) List() ([]SessionSummary, error) {
	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionSummary{}, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var summaries []SessionSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(fs.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var s SessionSummary
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}

		summaries = append(summaries, s)
	}

	return summaries, nil
}
