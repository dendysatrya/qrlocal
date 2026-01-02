// Package config provides configuration file support for qrlocal.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultConfigDir returns the default configuration directory (~/.qrlocal).
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".qrlocal"), nil
}

// DefaultConfigPath returns the default config file path (~/.qrlocal/config.yaml).
func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// ProviderConfig defines a tunnel provider configuration.
type ProviderConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	URLRegex string `yaml:"url_regex"`
}

// Config represents the qrlocal configuration file structure.
type Config struct {
	// Default settings
	DefaultProvider string `yaml:"default_provider"`
	CopyToClipboard bool   `yaml:"copy_to_clipboard"`
	QuietMode       bool   `yaml:"quiet_mode"`

	// Built-in provider settings
	Providers map[string]ProviderConfig `yaml:"providers"`

	// Custom providers defined by user
	CustomProviders map[string]ProviderConfig `yaml:"custom_providers"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultProvider: "localhost.run",
		CopyToClipboard: false,
		QuietMode:       false,
		Providers: map[string]ProviderConfig{
			"localhost.run": {
				Host:     "localhost.run",
				Port:     22,
				User:     "nokey",
				URLRegex: `https://[a-zA-Z0-9]+\.lhr\.life`,
			},
			"pinggy": {
				Host:     "a.pinggy.io",
				Port:     443,
				User:     "a",
				URLRegex: `https://[a-zA-Z0-9-]+\.a\.free\.pinggy\.link`,
			},
			"serveo": {
				Host:     "serveo.net",
				Port:     22,
				User:     "serveo",
				URLRegex: `Forwarding HTTP traffic from (https://[a-zA-Z0-9-]+\.(?:serveo\.net|serveousercontent\.com))`,
			},
			"tunnelto": {
				Host:     "tunnel.us.tunnel.to",
				Port:     22,
				User:     "tunnel",
				URLRegex: `https://[a-zA-Z0-9-]+\.tunnel\.to`,
			},
		},
		CustomProviders: map[string]ProviderConfig{},
	}
}

// Load reads and parses the configuration file.
// If the file doesn't exist, it returns the default configuration.
func Load(path string) (*Config, error) {
	// If no path specified, use default
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return DefaultConfig(), nil
		}
	}

	// Check if config file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Start with default config
	cfg := DefaultConfig()

	// Parse YAML
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save writes the configuration to the specified path.
func (c *Config) Save(path string) error {
	// If no path specified, use default
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetProvider returns the provider configuration by name.
// It checks built-in providers first, then custom providers.
func (c *Config) GetProvider(name string) (ProviderConfig, bool) {
	// Check built-in providers
	if p, ok := c.Providers[name]; ok {
		return p, true
	}

	// Check custom providers
	if p, ok := c.CustomProviders[name]; ok {
		return p, true
	}

	return ProviderConfig{}, false
}

// ListProviders returns all available provider names.
func (c *Config) ListProviders() []string {
	providers := make([]string, 0, len(c.Providers)+len(c.CustomProviders))

	for name := range c.Providers {
		providers = append(providers, name)
	}

	for name := range c.CustomProviders {
		providers = append(providers, name+" (custom)")
	}

	return providers
}

// InitConfig creates a new config file with default values.
func InitConfig(path string) error {
	cfg := DefaultConfig()
	return cfg.Save(path)
}

// Exists checks if a config file exists at the given path.
func Exists(path string) bool {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return false
		}
	}
	_, err := os.Stat(path)
	return err == nil
}
