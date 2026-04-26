package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultProvider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got %s", cfg.DefaultProvider)
	}

	if len(cfg.Providers) != 4 {
		t.Errorf("expected 4 providers, got %d", len(cfg.Providers))
	}

	// Check OpenAI config
	openai, ok := cfg.Providers["openai"]
	if !ok {
		t.Error("expected 'openai' provider in config")
	}
	if openai.Model != "gpt-4o-mini" {
		t.Errorf("expected OpenAI model 'gpt-4o-mini', got %s", openai.Model)
	}

	// Check Anthropic config
	anthropic, ok := cfg.Providers["anthropic"]
	if !ok {
		t.Error("expected 'anthropic' provider in config")
	}
	if anthropic.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected Anthropic model 'claude-sonnet-4-20250514', got %s", anthropic.Model)
	}
}

func TestConfig_GetProvider(t *testing.T) {
	cfg := DefaultConfig()

	p, ok := cfg.GetProvider("openai")
	if !ok {
		t.Fatal("expected to find 'openai' provider")
	}
	if p.Model != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got %s", p.Model)
	}

	_, ok = cfg.GetProvider("nonexistent")
	if ok {
		t.Error("expected not to find 'nonexistent' provider")
	}
}

func TestConfig_GetDefaultProvider(t *testing.T) {
	cfg := DefaultConfig()

	p, ok := cfg.GetDefaultProvider()
	if !ok {
		t.Fatal("expected to find default provider")
	}
	if p.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected default provider model 'claude-sonnet-4-20250514', got %s", p.Model)
	}
}

func TestLoader_SaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	loader := NewLoaderWithPath(configPath)

	// Save default config
	cfg := DefaultConfig()
	cfg.DefaultProvider = "openai"

	err := loader.Save(cfg)
	if err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify file exists
	if !loader.Exists() {
		t.Error("expected config file to exist after save")
	}

	// Load config back
	loaded, err := loader.LoadRaw()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.DefaultProvider != "openai" {
		t.Errorf("expected default provider 'openai', got %s", loaded.DefaultProvider)
	}
}

func TestLoader_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "config.yaml")

	loader := NewLoaderWithPath(configPath)

	// Should return default config when file doesn't exist
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}

	if cfg.DefaultProvider != "anthropic" {
		t.Errorf("expected default provider 'anthropic', got %s", cfg.DefaultProvider)
	}
}

func TestLoader_ExpandEnvVars(t *testing.T) {
	// Set test env var
	os.Setenv("TEST_API_KEY", "test-key-12345")
	defer os.Unsetenv("TEST_API_KEY")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write config with env var reference
	content := `default_provider: test
providers:
  test:
    api_key: ${TEST_API_KEY}
    model: test-model
    max_tokens: 1000
format:
  temperature: 0.5
  language: en
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := NewLoaderWithPath(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	testProvider, ok := cfg.GetProvider("test")
	if !ok {
		t.Fatal("expected to find 'test' provider")
	}

	if testProvider.APIKey != "test-key-12345" {
		t.Errorf("expected API key 'test-key-12345', got %s", testProvider.APIKey)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	if v := GetEnvOrDefault("TEST_VAR", "default"); v != "test-value" {
		t.Errorf("expected 'test-value', got %s", v)
	}

	if v := GetEnvOrDefault("NONEXISTENT_VAR", "default"); v != "default" {
		t.Errorf("expected 'default', got %s", v)
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"false", false},
		{"FALSE", false},
		{"0", false},
		{"no", false},
		{"", false},
		{"invalid", false},
	}

	for _, tc := range tests {
		os.Setenv("TEST_BOOL", tc.value)
		got := GetEnvBool("TEST_BOOL")
		if got != tc.expected {
			t.Errorf("GetEnvBool(%q): expected %v, got %v", tc.value, tc.expected, got)
		}
	}
	os.Unsetenv("TEST_BOOL")
}

func TestNewLoader(t *testing.T) {
	loader, err := NewLoader()
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}

	path := loader.ConfigPath()
	if path == "" {
		t.Error("expected non-empty config path")
	}

	// Should contain config.yaml
	if filepath.Base(path) != ConfigFileName {
		t.Errorf("expected config file name %s, got %s", ConfigFileName, filepath.Base(path))
	}
}

func TestLoader_Init(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	loader := NewLoaderWithPath(configPath)

	// Init should create file
	err := loader.Init()
	if err != nil {
		t.Fatalf("failed to init config: %v", err)
	}

	if !loader.Exists() {
		t.Error("expected config file to exist after init")
	}

	// Init again should fail
	err = loader.Init()
	if err == nil {
		t.Error("expected error when initializing existing config")
	}
}

func TestLoader_LoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	invalidYAML := "{{{{invalid yaml"
	if err := os.WriteFile(configPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := NewLoaderWithPath(configPath)
	_, err := loader.Load()
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestExpandEnvVars_UnsetVar(t *testing.T) {
	// Make sure the env var is unset
	os.Unsetenv("UNSET_VAR_FOR_TEST")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `default_provider: test
providers:
  test:
    api_key: ${UNSET_VAR_FOR_TEST}
    model: test-model
    max_tokens: 1000
format:
  temperature: 0.5
  language: en
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := NewLoaderWithPath(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	testProvider, ok := cfg.GetProvider("test")
	if !ok {
		t.Fatal("expected to find 'test' provider")
	}

	// Unset env var should result in empty string
	if testProvider.APIKey != "" {
		t.Errorf("expected empty API key for unset env var, got %s", testProvider.APIKey)
	}
}

func TestConfig_LLMRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoaderWithPath(filepath.Join(tmpDir, "config.yaml"))

	src := DefaultConfig()
	src.Parser = "upstage"
	src.LLM = LLMConfig{
		Enabled:  true,
		Provider: "openai",
		Model:    "gpt-4o-mini",
		BaseURL:  "http://localhost:8080",
		Timeout:  "5m",
	}

	if err := loader.Save(src); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := loader.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Parser != "upstage" {
		t.Errorf("Parser = %q, want %q", loaded.Parser, "upstage")
	}
	if !loaded.LLM.Enabled {
		t.Error("LLM.Enabled = false, want true")
	}
	if loaded.LLM.Provider != "openai" {
		t.Errorf("LLM.Provider = %q, want %q", loaded.LLM.Provider, "openai")
	}
	if loaded.LLM.Model != "gpt-4o-mini" {
		t.Errorf("LLM.Model = %q, want %q", loaded.LLM.Model, "gpt-4o-mini")
	}
	if loaded.LLM.BaseURL != "http://localhost:8080" {
		t.Errorf("LLM.BaseURL = %q, want %q", loaded.LLM.BaseURL, "http://localhost:8080")
	}
	if loaded.LLM.Timeout != "5m" {
		t.Errorf("LLM.Timeout = %q, want %q", loaded.LLM.Timeout, "5m")
	}
}
