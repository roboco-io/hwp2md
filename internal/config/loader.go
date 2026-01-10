package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigDirName is the name of the configuration directory.
	ConfigDirName = ".hwp2md"
	// ConfigFileName is the name of the configuration file.
	ConfigFileName = "config.yaml"
)

// envVarPattern matches ${VAR_NAME} patterns.
var envVarPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Loader handles configuration loading and saving.
type Loader struct {
	configDir  string
	configPath string
}

// NewLoader creates a new configuration loader.
func NewLoader() (*Loader, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ConfigDirName)
	configPath := filepath.Join(configDir, ConfigFileName)

	return &Loader{
		configDir:  configDir,
		configPath: configPath,
	}, nil
}

// NewLoaderWithPath creates a loader with a custom config path.
func NewLoaderWithPath(configPath string) *Loader {
	return &Loader{
		configDir:  filepath.Dir(configPath),
		configPath: configPath,
	}
}

// ConfigPath returns the configuration file path.
func (l *Loader) ConfigPath() string {
	return l.configPath
}

// Load reads and parses the configuration file.
func (l *Loader) Load() (*Config, error) {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables
	expanded := expandEnvVars(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// LoadRaw reads the configuration without expanding environment variables.
func (l *Loader) LoadRaw() (*Config, error) {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// Save writes the configuration to the file.
func (l *Loader) Save(cfg *Config) error {
	// Ensure config directory exists
	if err := os.MkdirAll(l.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(l.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Exists checks if the configuration file exists.
func (l *Loader) Exists() bool {
	_, err := os.Stat(l.configPath)
	return err == nil
}

// Init creates a default configuration file.
func (l *Loader) Init() error {
	if l.Exists() {
		return fmt.Errorf("config file already exists: %s", l.configPath)
	}
	return l.Save(DefaultConfig())
}

// expandEnvVars replaces ${VAR_NAME} with environment variable values.
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := strings.TrimSuffix(strings.TrimPrefix(match, "${"), "}")
		if value := os.Getenv(varName); value != "" {
			return value
		}
		// Return empty string if env var not set
		return ""
	})
}

// GetEnvOrDefault returns the environment variable value or a default.
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvBool returns true if the environment variable is set to "true" or "1".
func GetEnvBool(key string) bool {
	value := strings.ToLower(os.Getenv(key))
	return value == "true" || value == "1" || value == "yes"
}
