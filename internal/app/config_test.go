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

func TestValidBackendProvider(t *testing.T) {
	for _, p := range []string{"opencode", "codex", "claude", "claude-mirror", "OpenCode "} {
		if !ValidBackendProvider(p) {
			t.Fatalf("expected %q to be valid", p)
		}
	}
	for _, p := range []string{"codez", "", "gpt"} {
		if ValidBackendProvider(p) {
			t.Fatalf("expected %q to be invalid", p)
		}
	}
}

func TestResolveConfigPathReturnsDirWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "plums", "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got, err := ResolveConfigPath()
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}
	if got != dir {
		t.Fatalf("expected %q, got %q", dir, got)
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
		t.Fatalf("expected empty path (fall back to built-in defaults), got %q", got)
	}
}

func TestUserConfigGoPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got, err := UserConfigGoPath()
	if err != nil {
		t.Fatalf("user config path: %v", err)
	}
	want := filepath.Join(home, ".config", "plums", "config", "config.go")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestInitGlobalConfigSeedsConfigGo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := InitGlobalConfig()
	if err != nil {
		t.Fatalf("init config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "config.go")); err != nil {
		t.Fatalf("expected config.go seeded: %v", err)
	}
	// Idempotent: a second call must not error.
	if _, err := InitGlobalConfig(); err != nil {
		t.Fatalf("second init config: %v", err)
	}
}

func TestWriteDefaultConfigFileDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.go")
	if err := os.WriteFile(path, []byte("custom"), 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}
	if err := defaults.WriteDefault(dir, "config.go"); err == nil {
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
