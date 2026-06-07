// Package config loads configuration for the TUI coding agent manager.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for the TUI coding agent manager.
type Config struct {
	// DBPath is the path to the SQLite database file.
	DBPath string `json:"dbPath"`

	// ACPServerURL is the URL of the ACP server for coding agents.
	ACPServerURL string `json:"acpServerUrl"`

	// OpenAI holds OpenAI API configuration.
	OpenAI OpenAIConfig `json:"openai"`

	// DefaultWorkspace is the default workspace directory.
	DefaultWorkspace string `json:"defaultWorkspace,omitempty"`
}

// OpenAIConfig holds OpenAI API configuration.
type OpenAIConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"`
	Model   string `json:"model,omitempty"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DBPath:       filepath.Join(home, ".codeg", "codeg.db"),
		ACPServerURL: "http://localhost:3000/api/acp",
		OpenAI: OpenAIConfig{
			Model: "gpt-4o-mini",
		},
	}
}

// Load reads the configuration from a file, merging with defaults and
// environment variable overrides.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Load from file if it exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
			// File doesn't exist; use defaults
		} else {
			if err := json.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Environment variable overrides
	if v := os.Getenv("CODEG_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("CODEG_ACP_URL"); v != "" {
		cfg.ACPServerURL = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.OpenAI.APIKey = v
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		cfg.OpenAI.BaseURL = v
	}
	if v := os.Getenv("CODEG_LLM_MODEL"); v != "" {
		cfg.OpenAI.Model = v
	}
	if v := os.Getenv("CODEG_WORKSPACE"); v != "" {
		cfg.DefaultWorkspace = v
	}

	return cfg, nil
}

// EnsureDir ensures the configuration directory exists.
func (c *Config) EnsureDir() error {
	dir := filepath.Dir(c.DBPath)
	return os.MkdirAll(dir, 0755)
}
