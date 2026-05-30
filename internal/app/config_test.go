package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/app/defaults"
)

func TestLoadOpencodeServerURLFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[opencode]\nopencode_server_url = \"http://localhost:9999\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := LoadOpencodeServerURL(path, adapter.DefaultBaseURL)
	if err != nil {
		t.Fatalf("load server URL: %v", err)
	}
	if got != "http://localhost:9999" {
		t.Fatalf("expected configured URL, got %q", got)
	}
}

func TestLoadOpencodeServerURLDefault(t *testing.T) {
	got, err := LoadOpencodeServerURL(filepath.Join(t.TempDir(), "missing.toml"), adapter.DefaultBaseURL)
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	if got != adapter.DefaultBaseURL {
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

	got, err := ResolveCommandsConfigPath(layoutPath)
	if err != nil {
		t.Fatalf("resolve commands config: %v", err)
	}
	if got != commandsPath {
		t.Fatalf("expected %q, got %q", commandsPath, got)
	}
}

func TestResolveCommandsConfigPathDefaultsWhenMissing(t *testing.T) {
	got, err := ResolveCommandsConfigPath(filepath.Join(t.TempDir(), "layout.json"))
	if err != nil {
		t.Fatalf("resolve missing commands config: %v", err)
	}
	if got != "" {
		t.Fatalf("expected default command config, got %q", got)
	}
}

func TestResolveOpencodeConfigPathUsesLayoutConfigDir(t *testing.T) {
	layoutPath := filepath.Join("/tmp", "plums", "config", "layout.json")
	got := ResolveOpencodeConfigPath(layoutPath)
	want := filepath.Join("/tmp", "plums", "config", "config.toml")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWriteDefaultConfigFileDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("custom"), 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}
	if err := defaults.WriteDefault(dir, "config.toml"); err == nil {
		t.Fatal("expected existing config error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != "custom" {
		t.Fatalf("expected existing config to remain unchanged, got %q", string(data))
	}
}

func TestInitLocalConfigFilesCreatesProjectConfig(t *testing.T) {
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	dir, err := InitLocalConfigFiles()
	if err != nil {
		t.Fatalf("init local config: %v", err)
	}
	if dir != filepath.Join(".", ".agents", "plums", "config") {
		t.Fatalf("expected local config dir, got %q", dir)
	}
	for _, name := range []string{"config.toml", "layout.json", "commands.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}
