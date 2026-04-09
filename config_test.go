package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Server:       "https://example.com",
		AccessToken:  "at-123",
		RefreshToken: "rt-456",
		ExpiresAt:    time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC),
	}
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Server != cfg.Server {
		t.Errorf("server = %q", loaded.Server)
	}
	if loaded.AccessToken != cfg.AccessToken {
		t.Errorf("access_token = %q", loaded.AccessToken)
	}
	if loaded.RefreshToken != cfg.RefreshToken {
		t.Errorf("refresh_token = %q", loaded.RefreshToken)
	}
	if !loaded.ExpiresAt.Equal(cfg.ExpiresAt) {
		t.Errorf("expires_at = %v", loaded.ExpiresAt)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Server != "" {
		t.Errorf("server = %q, want empty", cfg.Server)
	}
}

func TestSaveConfigCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	cfg := Config{Server: "https://example.com"}
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("perm = %o, want 0600", perm)
	}
}

func TestConfigIsExpired(t *testing.T) {
	tests := []struct {
		name    string
		offset  time.Duration
		expired bool
	}{
		{"future", 2 * time.Hour, false},
		{"within_buffer", 30 * time.Second, true},
		{"past", -1 * time.Hour, true},
		{"zero", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{ExpiresAt: time.Now().Add(tt.offset)}
			if got := cfg.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v", got, tt.expired)
			}
		})
	}
}
