// Package config persists server settings that survive restarts.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is the on-disk server configuration.
type Config struct {
	Port int `json:"port"`
}

// DefaultPort is used when nothing else is configured.
const DefaultPort = 7945

func dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".rcontrol"
	}
	return filepath.Join(home, ".rcontrol")
}

// Path returns the config file path.
func Path() string { return filepath.Join(dir(), "config.json") }

// Load reads the config, falling back to defaults.
func Load() Config {
	c := Config{Port: DefaultPort}
	b, err := os.ReadFile(Path())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = DefaultPort
	}
	return c
}

// Save writes the config (0600).
func Save(c Config) error {
	if err := os.MkdirAll(dir(), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), append(b, '\n'), 0o600)
}
