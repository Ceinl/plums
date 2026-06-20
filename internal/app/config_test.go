package app

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ceinl/plums/internal/app/defaults"
)

const testDefaultOpencodeURL = "http://127.0.0.1:4096"

func TestLoadOpencodeServerURLFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[opencode]\nopencode_server_url = \"http://localhost:9999\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := LoadOpencodeServerURL(path, testDefaultOpencodeURL)
	if err != nil {
		t.Fatalf("load server URL: %v", err)
	}
	if got != "http://localhost:9999" {
		t.Fatalf("expected configured URL, got %q", got)
	}
}

func TestLoadOpencodeServerURLDefault(t *testing.T) {
	got, err := LoadOpencodeServerURL(filepath.Join(t.TempDir(), "missing.toml"), testDefaultOpencodeURL)
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	if got != testDefaultOpencodeURL {
		t.Fatalf("expected default URL, got %q", got)
	}
}

func TestLoadBackendProviderFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[backend]\nprovider = \"codex\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := LoadBackendProvider(path, "opencode")
	if err != nil {
		t.Fatalf("load backend provider: %v", err)
	}
	if got != "codex" {
		t.Fatalf("expected codex, got %q", got)
	}
}

func TestLoadBackendProviderDefault(t *testing.T) {
	got, err := LoadBackendProvider(filepath.Join(t.TempDir(), "missing.toml"), "opencode")
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}
	if got != "opencode" {
		t.Fatalf("expected opencode, got %q", got)
	}
}

func TestLoadBackendProviderRejectsUnknownProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[backend]\nprovider = \"codez\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := LoadBackendProvider(path, "opencode"); err == nil {
		t.Fatalf("expected unsupported provider error")
	}
}

func TestLoadBackendProviderRejectsUnknownFallback(t *testing.T) {
	if _, err := LoadBackendProvider(filepath.Join(t.TempDir(), "missing.toml"), "codez"); err == nil {
		t.Fatalf("expected unsupported fallback error")
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

func TestResolveConfigPathReturnsGlobalWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "plums", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := filepath.Join(dir, "layout.json")
	if err := os.WriteFile(want, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write layout: %v", err)
	}

	got, err := ResolveConfigPath()
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveConfigPathEmptyWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := ResolveConfigPath()
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty path (fall back to embedded defaults), got %q", got)
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

func TestDefaultCompiledConfigTemplate(t *testing.T) {
	data, err := defaults.Read("config.go")
	if err != nil {
		t.Fatalf("read default config.go: %v", err)
	}
	text := string(data)
	for _, want := range []string{"package config", "cfg.Use(Config)", "SplitLayoutPlugin", `layout.Named("split"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("default config.go missing %q:\n%s", want, text)
		}
	}
	// The template is not compiled by the test suite, so guard that it is at
	// least syntactically valid Go — a typo here only surfaces at `plums build`.
	if _, err := parser.ParseFile(token.NewFileSet(), "config.go", data, parser.AllErrors); err != nil {
		t.Fatalf("default config.go is not valid Go: %v", err)
	}
}
