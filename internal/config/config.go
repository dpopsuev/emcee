// Package config loads emcee configuration from ~/.config/emcee/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	appName    = "emcee"
	configFile = "config.yaml"
)

// Config is the top-level configuration.
type Config struct {
	Backends map[string]Backend `yaml:"backends"`
	Projects map[string]Project `yaml:"projects"`
}

// Backend holds credentials and connection details for an issue tracker.
type Backend struct {
	Type      string `yaml:"type,omitempty"` // backend type (e.g. "jenkins"); inferred from config key if empty
	APIKeyEnv string `yaml:"api_key_env"`    // env var name holding the API key
	TokenEnv  string `yaml:"token_env"`      // alternative env var name (GitHub, Jira)
	URL       string `yaml:"url,omitempty"`
	Email     string `yaml:"email,omitempty"`
	Team      string `yaml:"team,omitempty"`  // Linear team key or GitHub repo name
	Owner     string `yaml:"owner,omitempty"` // GitHub owner (org or user)
}

// ResolveType returns the backend type. If Type is set, returns it.
// Otherwise falls back to the config key (backward compat).
func (b Backend) ResolveType(key string) string {
	if b.Type != "" {
		return b.Type
	}
	return key
}

// ResolveKey reads the API key from the environment variable named in api_key_env or token_env.
func (b Backend) ResolveKey() string {
	if b.APIKeyEnv != "" {
		return os.Getenv(b.APIKeyEnv)
	}
	if b.TokenEnv != "" {
		return os.Getenv(b.TokenEnv)
	}
	return ""
}

// Project maps a friendly name to a backend and project identifier.
type Project struct {
	Backend string `yaml:"backend"`
	Project string `yaml:"project,omitempty"`
	Repo    string `yaml:"repo,omitempty"`
}

// Load reads configuration from the given path.
// If path is empty, it uses the default location ($XDG_CONFIG_HOME/emcee/config.yaml
// or ~/.config/emcee/config.yaml).
func Load(path string) (*Config, error) {
	if path == "" {
		path = defaultPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// Exists reports whether a config file exists at the default or given path.
func Exists(path string) bool {
	if path == "" {
		path = defaultPath()
	}
	_, err := os.Stat(path)
	return err == nil
}

func defaultPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, appName, configFile)
}
