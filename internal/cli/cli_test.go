package cli

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestSetVersion(t *testing.T) {
	oldVersion := version
	defer func() { version = oldVersion }()

	SetVersion("1.2.3")
	if version != "1.2.3" {
		t.Errorf("expected version '1.2.3', got '%s'", version)
	}
}

func TestRootCommand(t *testing.T) {
	// Test that root command exists and has expected properties
	if rootCmd.Use != "hwp2md [file]" {
		t.Errorf("expected Use 'hwp2md [file]', got '%s'", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestVersionCommand(t *testing.T) {
	if versionCmd.Use != "version" {
		t.Errorf("expected Use 'version', got '%s'", versionCmd.Use)
	}

	if versionCmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestProvidersCommand(t *testing.T) {
	if providersCmd.Use != "providers" {
		t.Errorf("expected Use 'providers', got '%s'", providersCmd.Use)
	}

	if providersCmd.Short == "" {
		t.Error("expected Short description to be set")
	}
}

func TestCheckProviderStatus(t *testing.T) {
	tests := []struct {
		name     string
		provider providerInfo
		envKey   string
		envValue string
		expected string
	}{
		{
			name: "ollama always available",
			provider: providerInfo{
				Name:   "ollama",
				EnvKey: "OLLAMA_HOST",
			},
			expected: "✓ 사용가능",
		},
		{
			name: "anthropic with key",
			provider: providerInfo{
				Name:   "anthropic",
				EnvKey: "ANTHROPIC_API_KEY",
			},
			envKey:   "ANTHROPIC_API_KEY",
			envValue: "test-key",
			expected: "✓ 설정됨",
		},
		{
			name: "openai without key",
			provider: providerInfo{
				Name:   "openai",
				EnvKey: "OPENAI_API_KEY",
			},
			envKey:   "OPENAI_API_KEY",
			envValue: "",
			expected: "✗ 미설정",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envKey != "" {
				oldVal := os.Getenv(tc.envKey)
				os.Setenv(tc.envKey, tc.envValue)
				defer os.Setenv(tc.envKey, oldVal)
			}

			result := checkProviderStatus(tc.provider)
			if result != tc.expected {
				t.Errorf("expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestConvertCommandFlags(t *testing.T) {
	if convertCmd.Use != "convert <file>" {
		t.Errorf("expected Use 'convert <file>', got '%s'", convertCmd.Use)
	}

	// Check flags exist
	flags := []string{"output", "llm", "provider", "model", "extract-images", "images-dir", "verbose", "quiet"}
	for _, flag := range flags {
		if convertCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag '%s' to exist", flag)
		}
	}
}

func TestExtractCommandFlags(t *testing.T) {
	if extractCmd.Use != "extract <file>" {
		t.Errorf("expected Use 'extract <file>', got '%s'", extractCmd.Use)
	}

	// Check flags exist
	flags := []string{"output", "format", "extract-images", "images-dir", "pretty"}
	for _, flag := range flags {
		if extractCmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected flag '%s' to exist", flag)
		}
	}
}

func TestConfigCommand(t *testing.T) {
	if configCmd.Use != "config" {
		t.Errorf("expected Use 'config', got '%s'", configCmd.Use)
	}

	// Check subcommands exist
	subcommands := []string{"show", "init", "set", "path"}
	for _, name := range subcommands {
		found := false
		for _, cmd := range configCmd.Commands() {
			if cmd.Use == name || cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected subcommand '%s' to exist", name)
		}
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "****"},
		{"12345678", "****"},
		{"sk-abcd1234efgh5678", "sk-a****5678"},
		{"AIzaSyD1234567890abcdefghijklmnop", "AIza****mnop"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := maskAPIKey(tc.input)
			if result != tc.expected {
				t.Errorf("maskAPIKey(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	if !contains(slice, "a") {
		t.Error("expected contains(slice, 'a') to be true")
	}

	if !contains(slice, "c") {
		t.Error("expected contains(slice, 'c') to be true")
	}

	if contains(slice, "d") {
		t.Error("expected contains(slice, 'd') to be false")
	}

	if contains([]string{}, "a") {
		t.Error("expected contains(empty, 'a') to be false")
	}
}

func TestDetectProviderFromModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		// Empty model defaults to anthropic
		{"", "anthropic"},

		// Anthropic models
		{"claude-3-opus", "anthropic"},
		{"claude-sonnet-4-20250514", "anthropic"},
		{"Claude-3-Haiku", "anthropic"},

		// OpenAI models
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"GPT-4-turbo", "openai"},
		{"o1-preview", "openai"},
		{"o1-mini", "openai"},
		{"o3-mini", "openai"},

		// Google Gemini models
		{"gemini-1.5-flash", "gemini"},
		{"gemini-1.5-pro", "gemini"},
		{"Gemini-2.0-flash", "gemini"},

		// Upstage Solar models
		{"solar-pro", "upstage"},
		{"solar-mini", "upstage"},
		{"Solar-Pro", "upstage"},

		// Unknown models default to Ollama
		{"llama3.2", "ollama"},
		{"mistral", "ollama"},
		{"qwen2.5", "ollama"},
		{"custom-model", "ollama"},
	}

	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := detectProviderFromModel(tc.model)
			if result != tc.expected {
				t.Errorf("detectProviderFromModel(%q) = %q, want %q", tc.model, result, tc.expected)
			}
		})
	}
}

func TestParseLLMTimeout(t *testing.T) {
	tests := []struct {
		name           string
		flag           string
		env            string
		want           time.Duration
		wantErr        bool
		wantErrContain string // optional substring expected in the error message
	}{
		{name: "both empty falls back to provider default", flag: "", env: "", want: 0},
		{name: "flag value parsed", flag: "5m", want: 5 * time.Minute},
		{name: "env value parsed when flag empty", env: "300s", want: 300 * time.Second},
		{name: "flag overrides env", flag: "10m", env: "30s", want: 10 * time.Minute},
		{name: "compound duration", flag: "10m30s", want: 10*time.Minute + 30*time.Second},
		{name: "whitespace trimmed", flag: "  2m ", want: 2 * time.Minute},
		{name: "whitespace flag falls back to env", flag: "  ", env: "5m", want: 5 * time.Minute},
		{name: "whitespace env returns provider default", env: "\t", want: 0},
		{name: "invalid flag is reported as flag", flag: "abc", env: "5m", wantErr: true, wantErrContain: "--timeout"},
		{name: "invalid env is reported as env", env: "abc", wantErr: true, wantErrContain: "HWP2MD_TIMEOUT"},
		{name: "invalid env returns error", env: "5", wantErr: true},
		{name: "zero rejected", flag: "0s", wantErr: true, wantErrContain: "양수"},
		{name: "negative rejected", flag: "-1s", wantErr: true, wantErrContain: "양수"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLLMTimeout(tc.flag, tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseLLMTimeout(%q, %q) expected error, got nil", tc.flag, tc.env)
				}
				if tc.wantErrContain != "" && !strings.Contains(err.Error(), tc.wantErrContain) {
					t.Errorf("parseLLMTimeout(%q, %q) error %q does not contain %q",
						tc.flag, tc.env, err.Error(), tc.wantErrContain)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseLLMTimeout(%q, %q) unexpected error: %v", tc.flag, tc.env, err)
			}
			if got != tc.want {
				t.Errorf("parseLLMTimeout(%q, %q) = %v, want %v", tc.flag, tc.env, got, tc.want)
			}
		})
	}
}
