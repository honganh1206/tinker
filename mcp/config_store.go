package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConfigStore interface {
	Save(config ServerConfig) error
	Load(id string) (ServerConfig, error)
	List() ([]ServerConfig, error)
	Delete(id string) error
}

type FileConfigStore struct {
	dir string
}

func NewFileConfigStore(dir string) *FileConfigStore {
	_ = os.MkdirAll(dir, 0o755)
	return &FileConfigStore{dir: dir}
}

func (s *FileConfigStore) Save(config ServerConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if config.ID == "" {
		return fmt.Errorf("failed to write config file: ID must not empty")
	}
	path := filepath.Join(s.dir, config.ID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

func (s *FileConfigStore) Load(id string) (ServerConfig, error) {
	path := filepath.Join(s.dir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ServerConfig{}, fmt.Errorf("config %s not found", id)
		}
		return ServerConfig{}, fmt.Errorf("failed to read config file: %w", err)
	}
	var config ServerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return ServerConfig{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return config, nil
}

func (s *FileConfigStore) List() ([]ServerConfig, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ServerConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}
	var configs []ServerConfig
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var config ServerConfig
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}
		configs = append(configs, config)
	}
	return configs, nil
}

func (s *FileConfigStore) Delete(id string) error {
	path := filepath.Join(s.dir, id+".json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("config %s not found", id)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete config file: %w", err)
	}
	return nil
}
