package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	p := DefaultConfigPath()
	if p == "" {
		t.Error("DefaultConfigPath() returned empty")
	}
}

func TestDefaultKeysDir(t *testing.T) {
	d := DefaultKeysDir()
	if d == "" {
		t.Error("DefaultKeysDir() returned empty")
	}
}

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs() error: %v", err)
	}

	if _, err := os.Stat(DefaultConfigDir()); err != nil {
		t.Errorf("config dir not created: %v", err)
	}
	if _, err := os.Stat(DefaultKeysDir()); err != nil {
		t.Errorf("keys dir not created: %v", err)
	}
}

func TestLoadNonexistent(t *testing.T) {
	cfg, err := Load("/tmp/nonexistent-config-test.toml")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.GatewayEndpoint != "http://localhost:8080" {
		t.Errorf("GatewayEndpoint = %q, want http://localhost:8080", cfg.GatewayEndpoint)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &Config{
		GatewayEndpoint: "http://gateway:8080",
		AgentID:         "agent-123",
		AgentName:       "test-agent",
		Owner:           "org:eng",
		KeyPath:         "/tmp/test.key",
		TimeoutSecs:     60,
		APIKey:          "ak-123",
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.GatewayEndpoint != cfg.GatewayEndpoint {
		t.Errorf("GatewayEndpoint = %q, want %q", loaded.GatewayEndpoint, cfg.GatewayEndpoint)
	}
	if loaded.AgentID != cfg.AgentID {
		t.Errorf("AgentID = %q, want %q", loaded.AgentID, cfg.AgentID)
	}
	if loaded.AgentName != cfg.AgentName {
		t.Errorf("AgentName = %q, want %q", loaded.AgentName, cfg.AgentName)
	}
	if loaded.Owner != cfg.Owner {
		t.Errorf("Owner = %q, want %q", loaded.Owner, cfg.Owner)
	}
	if loaded.KeyPath != cfg.KeyPath {
		t.Errorf("KeyPath = %q, want %q", loaded.KeyPath, cfg.KeyPath)
	}
	if loaded.TimeoutSecs != cfg.TimeoutSecs {
		t.Errorf("TimeoutSecs = %d, want %d", loaded.TimeoutSecs, cfg.TimeoutSecs)
	}
}

func TestParseTOMLBasic(t *testing.T) {
	data := []byte(`[agent]
name = "my-agent"
agent_id = "abc-123"

[gateway]
endpoint = "http://test:9090"
timeout_secs = 45
`)
	cfg := &Config{}
	if err := parseTOML(data, cfg); err != nil {
		t.Fatalf("parseTOML() error: %v", err)
	}
	if cfg.AgentName != "my-agent" {
		t.Errorf("AgentName = %q, want my-agent", cfg.AgentName)
	}
	if cfg.AgentID != "abc-123" {
		t.Errorf("AgentID = %q, want abc-123", cfg.AgentID)
	}
	if cfg.GatewayEndpoint != "http://test:9090" {
		t.Errorf("GatewayEndpoint = %q, want http://test:9090", cfg.GatewayEndpoint)
	}
	if cfg.TimeoutSecs != 45 {
		t.Errorf("TimeoutSecs = %d, want 45", cfg.TimeoutSecs)
	}
}