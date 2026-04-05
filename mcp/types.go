package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const mcpConfigFile = "mcp_servers.json"

type ServerConfig struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

func SaveConfigs(configs []ServerConfig) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	tinkerDir := filepath.Join(configDir, "tinker")
	if err := os.MkdirAll(tinkerDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(tinkerDir, mcpConfigFile)
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

func LoadConfigs() ([]ServerConfig, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, "tinker", mcpConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return []ServerConfig{}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var configs []ServerConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, err
	}

	return configs, nil
}
