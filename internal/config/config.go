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
	// When empty, codeg manages its own opencode serve process.
	ACPServerURL string `json:"acpServerUrl"`

	// ACP holds configuration for the managed ACP server.
	ACP ACPConfig `json:"acp"`

	// OpenAI holds OpenAI API configuration.
	OpenAI OpenAIConfig `json:"openai"`

	// Planning holds configuration for the LLM-based task planner.
	Planning PlanningConfig `json:"planning"`

	// DefaultWorkspace is the default workspace directory.
	DefaultWorkspace string `json:"defaultWorkspace,omitempty"`
}

// ACPConfig holds configuration for the managed ACP server process.
type ACPConfig struct {
	Port     int    `json:"port"`     // 0 = random port
	Provider string `json:"provider"` // e.g. "anthropic", "openai"
	Model    string `json:"model"`    // model override
}

// OpenAIConfig holds OpenAI API configuration.
type OpenAIConfig struct {
	APIKey  string `json:"apiKey"`
	BaseURL string `json:"baseUrl,omitempty"`
	Model   string `json:"model,omitempty"`
}

// PlanningConfig holds configuration for LLM-based task decomposition.
type PlanningConfig struct {
	Model   string `json:"model"`   // model for planning (defaults to OpenAI model)
	Enabled bool   `json:"enabled"` // enable/disable LLM planning
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DBPath:       filepath.Join(home, ".codeg", "codeg.db"),
		ACPServerURL: "", // empty = manage own opencode serve process
		OpenAI: OpenAIConfig{
			Model: "gpt-4o-mini",
		},
		Planning: PlanningConfig{
			Enabled: true,
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
	if v := os.Getenv("CODEG_ACP_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.ACP.Port)
	}
	if v := os.Getenv("CODEG_ACP_PROVIDER"); v != "" {
		cfg.ACP.Provider = v
	}
	if v := os.Getenv("CODEG_PLANNING_MODEL"); v != "" {
		cfg.Planning.Model = v
	}
	if v := os.Getenv("CODEG_PLANNING_ENABLED"); v != "" {
		cfg.Planning.Enabled = v == "true" || v == "1"
	}

	return cfg, nil
}

// EnsureDir ensures the configuration directory exists.
func (c *Config) EnsureDir() error {
	dir := filepath.Dir(c.DBPath)
	return os.MkdirAll(dir, 0755)
}
