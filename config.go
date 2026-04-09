package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Config holds persistent CLI configuration and OAuth tokens.
type Config struct {
	Server       string    `json:"server"`
	AccessToken  string    `json:"access_token,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// IsExpired returns true if the access token has expired or is within 60s of expiry.
func (c Config) IsExpired() bool {
	return time.Now().After(c.ExpiresAt.Add(-60 * time.Second))
}

// DefaultDir returns ~/.odyssey.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".odyssey"
	}
	return filepath.Join(home, ".odyssey")
}

// LoadConfig reads config.json from dir. Returns empty Config if file is absent.
func LoadConfig(dir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// SaveConfig writes config.json to dir with 0600 permissions.
// Creates dir with 0700 if needed.
func SaveConfig(dir string, cfg Config) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
}
