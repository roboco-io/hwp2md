// Package config manages application configuration.
package config

// Config represents the application configuration.
type Config struct {
	// Parser selects the document parser: "native" (default) or "upstage".
	Parser string `yaml:"parser,omitempty"`
	// LLM contains the active LLM settings used by the convert command.
	LLM LLMConfig `yaml:"llm,omitempty"`

	// DefaultProvider/Providers/Format are retained for backward compatibility
	// with existing config files and the legacy `format.*` set keys.
	DefaultProvider string              `yaml:"default_provider,omitempty"`
	Providers       map[string]Provider `yaml:"providers,omitempty"`
	Format          FormatConfig        `yaml:"format,omitempty"`
}

// LLMConfig holds settings for the LLM stage of the convert command.
// Empty fields fall back to environment variables and provider defaults.
type LLMConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Provider string `yaml:"provider,omitempty"`
	Model    string `yaml:"model,omitempty"`
	BaseURL  string `yaml:"base_url,omitempty"`
	// Timeout is a Go duration string (e.g. "5m", "300s"). Empty means default.
	Timeout string `yaml:"timeout,omitempty"`
}

// Provider represents an LLM provider configuration.
type Provider struct {
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	MaxTokens int    `yaml:"max_tokens"`
	Endpoint  string `yaml:"endpoint,omitempty"` // for Ollama or custom endpoints
}

// FormatConfig contains formatting options.
type FormatConfig struct {
	Temperature float64 `yaml:"temperature"`
	Language    string  `yaml:"language"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultProvider: "anthropic",
		Providers: map[string]Provider{
			"openai": {
				APIKey:    "${OPENAI_API_KEY}",
				Model:     "gpt-4o-mini",
				MaxTokens: 4096,
			},
			"anthropic": {
				APIKey:    "${ANTHROPIC_API_KEY}",
				Model:     "claude-sonnet-4-20250514",
				MaxTokens: 4096,
			},
			"gemini": {
				APIKey:    "${GOOGLE_API_KEY}",
				Model:     "gemini-1.5-flash",
				MaxTokens: 4096,
			},
			"ollama": {
				Endpoint:  "http://localhost:11434",
				Model:     "llama3.2",
				MaxTokens: 4096,
			},
		},
		Format: FormatConfig{
			Temperature: 0.3,
			Language:    "ko",
		},
	}
}

// GetProvider returns the provider configuration by name.
func (c *Config) GetProvider(name string) (*Provider, bool) {
	p, ok := c.Providers[name]
	if !ok {
		return nil, false
	}
	return &p, true
}

// GetDefaultProvider returns the default provider configuration.
func (c *Config) GetDefaultProvider() (*Provider, bool) {
	return c.GetProvider(c.DefaultProvider)
}
