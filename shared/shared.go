package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerIP        string            `json:"serverIP"`
	ServerPort      int               `json:"serverPort"`
}

type RequestType int

const (
	RequestTypeClipboard RequestType = iota
	RequestTypeRun
)

type Request struct {
	Type RequestType `json:"type"`
	Data string      `json:"data,omitempty"`
	Args []string      `json:"args,omitempty"`
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, "clipd.json")
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file at %s: %w", configPath, err)
	}
	defer configFile.Close()
	var config Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	if config.ServerIP == "" {
		return nil, fmt.Errorf("serverIP is required in config")
	}
	if config.ServerPort <= 0 || config.ServerPort > 65535 {
		return nil, fmt.Errorf("serverPort must be between 1 and 65535")
	}
	return &config, nil
}
