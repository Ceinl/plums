package scaffold

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewPluginSeedsConfigModuleAndPlugin(t *testing.T) {
	configDir := t.TempDir()

	result, err := NewPlugin(PluginOptions{
		ConfigDir:    configDir,
		Name:         "hello-world",
		PlumsVersion: "v1.2.3",
	})
	if err != nil {
		t.Fatalf("NewPlugin() error = %v", err)
	}
	if result.PackageName != "hello_world" {
		t.Fatalf("PackageName = %q, want hello_world", result.PackageName)
	}
	if result.ImportPath != "github.com/Ceinl/plums-user/plugins/hello-world" {
		t.Fatalf("ImportPath = %q", result.ImportPath)
	}
	for _, path := range []string{
		filepath.Join(configDir, "go.mod"),
		filepath.Join(configDir, "config.go"),
		filepath.Join(configDir, "plugins", "hello-world", "hello_world.go"),
		filepath.Join(configDir, "plugins", "hello-world", "hello_world_test.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s: %v", path, err)
		}
	}
	goMod, err := os.ReadFile(filepath.Join(configDir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "require github.com/Ceinl/plums v1.2.3") {
		t.Fatalf("go.mod missing plums require:\n%s", goMod)
	}
	assertParseGo(t, filepath.Join(configDir, "plugins", "hello-world", "hello_world.go"))
	assertParseGo(t, filepath.Join(configDir, "plugins", "hello-world", "hello_world_test.go"))
}

func TestNewPluginRejectsExistingPlugin(t *testing.T) {
	configDir := t.TempDir()
	if _, err := NewPlugin(PluginOptions{ConfigDir: configDir, Name: "demo"}); err != nil {
		t.Fatalf("first NewPlugin() error = %v", err)
	}
	if _, err := NewPlugin(PluginOptions{ConfigDir: configDir, Name: "demo"}); err == nil {
		t.Fatal("expected duplicate plugin to fail")
	}
}

func TestPluginNamesSanitizePackageIdentifier(t *testing.T) {
	dirName, packageName, err := pluginNames("123-func")
	if err != nil {
		t.Fatalf("pluginNames() error = %v", err)
	}
	if dirName != "123-func" {
		t.Fatalf("dirName = %q", dirName)
	}
	if packageName != "plugin_123_func" {
		t.Fatalf("packageName = %q", packageName)
	}
}

func assertParseGo(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), path, data, parser.AllErrors); err != nil {
		t.Fatalf("%s is not valid Go: %v\n%s", path, err, data)
	}
}
