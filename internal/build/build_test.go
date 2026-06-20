package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateMainImportsUserConfigAndRuntime(t *testing.T) {
	got := GenerateMain(Options{Version: "1.2.3", Commit: "abc", BuildDate: "today"})
	for _, want := range []string{
		`_ "github.com/Ceinl/plums-user"`,
		`plumsruntime "github.com/Ceinl/plums/plums/runtime"`,
		`version = "1.2.3"`,
		`commit = "abc"`,
		`buildDate = "today"`,
		`plumsruntime.Main(version, commit, buildDate)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated main missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateGoModUsesReplaceWhenPlumsDirProvided(t *testing.T) {
	got := GenerateGoMod(Options{
		PlumsVersion:   "v1.2.3",
		PlumsModuleDir: "/tmp/plums",
	})
	for _, want := range []string{
		"module github.com/Ceinl/plums-user",
		"go 1.26.2",
		"require github.com/Ceinl/plums v1.2.3",
		"replace github.com/Ceinl/plums => /tmp/plums",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated go.mod missing %q:\n%s", want, got)
		}
	}
}

func TestCopyTreeCopiesRegularFilesAndSkipsBuildArtifacts(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	mustWrite(t, filepath.Join(src, "config.go"), "package config\n")
	mustWrite(t, filepath.Join(src, "plugins", "github", "github.go"), "package github\n")
	mustWrite(t, filepath.Join(src, "bin", "plums"), "binary")
	mustWrite(t, filepath.Join(src, ".cache", "state"), "cache")

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("copyTree() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "config.go")); err != nil {
		t.Fatalf("config.go not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "plugins", "github", "github.go")); err != nil {
		t.Fatalf("plugin file not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "bin", "plums")); !os.IsNotExist(err) {
		t.Fatalf("bin artifact copied, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".cache", "state")); !os.IsNotExist(err) {
		t.Fatalf("cache artifact copied, stat err = %v", err)
	}
}

func TestDefaultConfigDirMatchesInitConfigLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := DefaultConfigDir()
	if err != nil {
		t.Fatalf("DefaultConfigDir() error = %v", err)
	}
	want := filepath.Join(home, ".config", "plums", "config")
	if got != want {
		t.Fatalf("DefaultConfigDir() = %q, want %q", got, want)
	}
}

func TestCopyGoSumForLocalReplace(t *testing.T) {
	moduleDir := t.TempDir()
	workDir := t.TempDir()
	mustWrite(t, filepath.Join(moduleDir, "go.sum"), "example.com/mod v1.0.0 h1:abc\n")

	if err := copyGoSum(moduleDir, workDir); err != nil {
		t.Fatalf("copyGoSum() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(workDir, "go.sum"))
	if err != nil {
		t.Fatalf("read copied go.sum: %v", err)
	}
	if string(data) != "example.com/mod v1.0.0 h1:abc\n" {
		t.Fatalf("go.sum = %q", string(data))
	}
}

func TestCacheKeyTracksConfigSourcesAndIgnoresBuildArtifacts(t *testing.T) {
	configDir := t.TempDir()
	mustWrite(t, filepath.Join(configDir, "config.go"), "package config\n")

	key1, err := CacheKey(configDir, Options{Version: "1"})
	if err != nil {
		t.Fatalf("CacheKey() error = %v", err)
	}
	mustWrite(t, filepath.Join(configDir, ".cache", "state"), "ignored")
	key2, err := CacheKey(configDir, Options{Version: "1"})
	if err != nil {
		t.Fatalf("CacheKey() with cache artifact error = %v", err)
	}
	if key1 != key2 {
		t.Fatalf("cache artifact changed key: %q -> %q", key1, key2)
	}

	mustWrite(t, filepath.Join(configDir, "plugins", "demo", "demo.go"), "package demo\n")
	key3, err := CacheKey(configDir, Options{Version: "1"})
	if err != nil {
		t.Fatalf("CacheKey() with plugin source error = %v", err)
	}
	if key3 == key1 {
		t.Fatal("plugin source change did not change cache key")
	}
}

func TestBuildOutputPathUsesHashCacheByDefault(t *testing.T) {
	configDir := t.TempDir()
	cacheDir := t.TempDir()
	t.Setenv("HOME", cacheDir)
	mustWrite(t, filepath.Join(configDir, "config.go"), "package config\n")

	key, err := CacheKey(configDir, Options{})
	if err != nil {
		t.Fatalf("CacheKey() error = %v", err)
	}
	output, cached, err := buildOutputPath(configDir, Options{})
	if err != nil {
		t.Fatalf("buildOutputPath() error = %v", err)
	}
	if !cached {
		t.Fatal("buildOutputPath() cached = false, want true for default output")
	}
	if !strings.Contains(filepath.ToSlash(output), key) {
		t.Fatalf("default output %q does not contain cache key %q", output, key)
	}

	explicit, cached, err := buildOutputPath(configDir, Options{OutputPath: filepath.Join(configDir, "bin", "plums")})
	if err != nil {
		t.Fatalf("buildOutputPath() explicit error = %v", err)
	}
	if cached {
		t.Fatal("explicit output path should not use cache hit short-circuit")
	}
	if filepath.Base(explicit) != "plums" {
		t.Fatalf("explicit output = %q", explicit)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
