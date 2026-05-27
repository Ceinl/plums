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
