package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	home, _ := os.UserHomeDir()
	expectedDB := filepath.Join(home, ".codeg", "codeg.db")

	if cfg.DBPath != expectedDB {
		t.Errorf("expected DBPath %q, got %q", expectedDB, cfg.DBPath)
	}
	if cfg.ACPServerURL != "" {
		t.Errorf("expected empty ACPServerURL (managed mode), got %q", cfg.ACPServerURL)
	}
	if cfg.OpenAI.Model != "gpt-4o-mini" {
		t.Errorf("expected model gpt-4o-mini, got %q", cfg.OpenAI.Model)
	}
	if !cfg.Planning.Enabled {
		t.Error("expected Planning.Enabled to be true by default")
	}
}

func TestDefaultConfig_StructFields(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ACP.Port != 4200 {
		t.Errorf("expected ACP port 4200 (default), got %d", cfg.ACP.Port)
	}
	if cfg.ACP.Provider != "" {
		t.Errorf("expected empty ACP provider, got %q", cfg.ACP.Provider)
	}
	if cfg.OpenAI.APIKey != "" {
		t.Errorf("expected empty API key, got %q", cfg.OpenAI.APIKey)
	}
}

func TestLoad_EnvOverrides(t *testing.T) {
	// Set env vars
	os.Setenv("CODEG_DB_PATH", "/tmp/test-codeg.db")
	os.Setenv("CODEG_ACP_URL", "http://localhost:9999/api/acp")
	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("OPENAI_BASE_URL", "https://api.deepseek.com")
	os.Setenv("CODEG_LLM_MODEL", "deepseek-chat")
	os.Setenv("CODEG_WORKSPACE", "/home/test/projects")
	os.Setenv("CODEG_ACP_PORT", "4200")
	os.Setenv("CODEG_ACP_PROVIDER", "anthropic")
	os.Setenv("CODEG_PLANNING_MODEL", "deepseek-reasoner")
	os.Setenv("CODEG_PLANNING_ENABLED", "false")

	defer func() {
		os.Unsetenv("CODEG_DB_PATH")
		os.Unsetenv("CODEG_ACP_URL")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_BASE_URL")
		os.Unsetenv("CODEG_LLM_MODEL")
		os.Unsetenv("CODEG_WORKSPACE")
		os.Unsetenv("CODEG_ACP_PORT")
		os.Unsetenv("CODEG_ACP_PROVIDER")
		os.Unsetenv("CODEG_PLANNING_MODEL")
		os.Unsetenv("CODEG_PLANNING_ENABLED")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DBPath != "/tmp/test-codeg.db" {
		t.Errorf("DBPath: got %q", cfg.DBPath)
	}
	if cfg.ACPServerURL != "http://localhost:9999/api/acp" {
		t.Errorf("ACPServerURL: got %q", cfg.ACPServerURL)
	}
	if cfg.OpenAI.APIKey != "sk-test-key" {
		t.Errorf("APIKey: got %q", cfg.OpenAI.APIKey)
	}
	if cfg.OpenAI.BaseURL != "https://api.deepseek.com" {
		t.Errorf("BaseURL: got %q", cfg.OpenAI.BaseURL)
	}
	if cfg.OpenAI.Model != "deepseek-chat" {
		t.Errorf("Model: got %q", cfg.OpenAI.Model)
	}
	if cfg.DefaultWorkspace != "/home/test/projects" {
		t.Errorf("Workspace: got %q", cfg.DefaultWorkspace)
	}
	if cfg.ACP.Port != 4200 {
		t.Errorf("ACP Port: got %d", cfg.ACP.Port)
	}
	if cfg.ACP.Provider != "anthropic" {
		t.Errorf("ACP Provider: got %q", cfg.ACP.Provider)
	}
	if cfg.Planning.Model != "deepseek-reasoner" {
		t.Errorf("Planning Model: got %q", cfg.Planning.Model)
	}
	if cfg.Planning.Enabled {
		t.Error("Planning.Enabled should be false")
	}
}

func TestLoad_PlanningEnabledTrue(t *testing.T) {
	os.Setenv("CODEG_PLANNING_ENABLED", "1")
	defer os.Unsetenv("CODEG_PLANNING_ENABLED")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !cfg.Planning.Enabled {
		t.Error("Planning.Enabled should be true with env=1")
	}
}

func TestLoad_WithFilePath(t *testing.T) {
	// Load should work even with a non-existent file (falls back to defaults)
	cfg, err := Load("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("Load with nonexistent file should not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestEnsureDir(t *testing.T) {
	cfg := &Config{DBPath: "/tmp/codeg-test-dir/codeg.db"}
	err := cfg.EnsureDir()
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}
	// Cleanup
	os.RemoveAll("/tmp/codeg-test-dir")
}

func TestACPConfig(t *testing.T) {
	cfg := ACPConfig{Port: 8080, Provider: "openai", Model: "gpt-4"}
	if cfg.Port != 8080 {
		t.Error("Port mismatch")
	}
	if cfg.Provider != "openai" {
		t.Error("Provider mismatch")
	}
}

func TestPlanningConfig(t *testing.T) {
	cfg := PlanningConfig{Model: "deepseek-chat", Enabled: true}
	if cfg.Model != "deepseek-chat" {
		t.Error("Model mismatch")
	}
	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestOpenAIConfig(t *testing.T) {
	cfg := OpenAIConfig{APIKey: "sk-abc", BaseURL: "https://api.openai.com", Model: "gpt-4"}
	if cfg.APIKey != "sk-abc" {
		t.Error("APIKey mismatch")
	}
	if cfg.BaseURL != "https://api.openai.com" {
		t.Error("BaseURL mismatch")
	}
}
