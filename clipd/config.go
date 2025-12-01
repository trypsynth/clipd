package clipd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ServerIP      string            `json:"serverIP"`
	ServerPort    int               `json:"serverPort"`
	DriveMappings map[string]string `json:"driveMappings,omitempty"`
	Password      string            `json:"password,omitempty"`
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	configPath := filepath.Join(homeDir, ".clipd")
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file at %s: %w", configPath, err)
	}
	defer configFile.Close()
	var config Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}
	config.ServerIP = os.ExpandEnv(config.ServerIP)
	config.Password = os.ExpandEnv(config.Password)
	for key, value := range config.DriveMappings {
		config.DriveMappings[key] = os.ExpandEnv(value)
	}
	if config.ServerIP == "" {
		return nil, fmt.Errorf("serverIP is required in config")
	}
	if config.ServerPort <= 0 || config.ServerPort > 65535 {
		return nil, fmt.Errorf("serverPort must be between 1 and 65535")
	}
	return &config, nil
}

func ResolvePath(path string, mappings map[string]string) string {
	if len(mappings) == 0 {
		return path
	}
	expandedPath := expandHomePath(path)
	normalizedPath := strings.ReplaceAll(expandedPath, "\\", "/")
	var bestMatch string
	var bestDrive string
	for drive, linuxPath := range mappings {
		normalizedLinuxPath := strings.ReplaceAll(linuxPath, "\\", "/")
		if strings.HasPrefix(normalizedPath, normalizedLinuxPath) {
			// Ensure it's an exact match or followed by a path separator.
			remainder := normalizedPath[len(normalizedLinuxPath):]
			if remainder == "" || strings.HasPrefix(remainder, "/") {
				if len(normalizedLinuxPath) > len(bestMatch) {
					bestMatch = normalizedLinuxPath
					bestDrive = drive
				}
			}
		}
	}
	if bestMatch != "" {
		relativePath := strings.TrimPrefix(normalizedPath, bestMatch)
		if strings.HasPrefix(relativePath, "/") {
			relativePath = relativePath[1:]
		}
		drive := strings.TrimSuffix(bestDrive, ":")
		if relativePath == "" {
			return drive + ":\\"
		}
		relativePath = strings.ReplaceAll(relativePath, "/", "\\")
		return drive + ":\\" + relativePath
	}
	return path
}

func expandHomePath(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func ResolveArgs(args []string, mappings map[string]string) []string {
	resolved := make([]string, len(args))
	for i, arg := range args {
		resolved[i] = ResolvePath(arg, mappings)
	}
	return resolved
}

func GetWorkingDir(mappings map[string]string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return ResolvePath(cwd, mappings), nil
}
