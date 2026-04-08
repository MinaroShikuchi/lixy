package controller

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ControllerConfig holds configuration for the controller (lixy)
type ControllerConfig struct {
	Name       string `yaml:"name"`
	Port       int    `yaml:"port"`
	LogLevel   string `yaml:"log_level"`
	SocketPath string `yaml:"socket_path"`
	Database   string `yaml:"database"`
	LogFile    string `yaml:"log_file"`
	Health     struct {
		CheckInterval string `yaml:"check_interval"` // e.g. "5m"
	} `yaml:"health"`
}

// LoadControllerConfig loads controller configuration from a YAML file.
// If configPath is empty, it searches default candidate locations.
func LoadControllerConfig(configPath string) (*ControllerConfig, error) {
	path := configPath

	cfg := &ControllerConfig{
		// Set defaults
		Name:       "Lixy Controller",
		Port:       8080,
		LogLevel:   "info",
		SocketPath: "/tmp/lixy.sock",
		Database:   "lixy.db",
		LogFile:    "./lixy.log",
	}
	cfg.Health.CheckInterval = "5m"

	if path == "" {
		// Try default locations
		candidates := []string{
			"/etc/lixy/lixy.yaml",
			"./lixy.yaml",
			"./config/lixy.yaml",
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
		if path == "" {
			fmt.Println("No config provided, using default configuration")
			return cfg, nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("Error reading config file at", path, ":", err)
		fmt.Println("Using default configuration")
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		fmt.Println("Error parsing config file at", path, ":", err)
		fmt.Println("Using default configuration")
		return cfg, nil
	}

	return cfg, nil
}

// SaveControllerConfig writes controller configuration to a YAML file
func SaveControllerConfig(cfg *ControllerConfig, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
