package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOpencodeServerURLFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[opencode]\nopencode_server_url = \"http://localhost:9999\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := loadOpencodeServerURL(path)
	if err != nil {
		t.Fatalf("load server URL: %v", err)
	}
	if got != "http://localhost:9999" {
		t.Fatalf("expected configured URL, got %q", got)
	}
}

func TestLoadOpencodeServerURLDefault(t *testing.T) {
	got, err := loadOpencodeServerURL(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	if got != "http://127.0.0.1:4096" {
		t.Fatalf("expected default URL, got %q", got)
	}
}

func TestResolveCommandsConfigPath(t *testing.T) {
	dir := t.TempDir()
	layoutPath := filepath.Join(dir, "layout.json")
	commandsPath := filepath.Join(dir, "commands.json")
	if err := os.WriteFile(commandsPath, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write commands config: %v", err)
	}

	got, err := resolveCommandsConfigPath(layoutPath)
	if err != nil {
		t.Fatalf("resolve commands config: %v", err)
	}
	if got != commandsPath {
		t.Fatalf("expected %q, got %q", commandsPath, got)
	}
}

func TestResolveCommandsConfigPathDefaultsWhenMissing(t *testing.T) {
	got, err := resolveCommandsConfigPath(filepath.Join(t.TempDir(), "layout.json"))
	if err != nil {
		t.Fatalf("resolve missing commands config: %v", err)
	}
	if got != "" {
		t.Fatalf("expected default command config, got %q", got)
	}
}
