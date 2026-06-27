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

func TestWirePluginAddsImportAndPluginEntry(t *testing.T) {
	configDir := t.TempDir()
	if _, err := NewPlugin(PluginOptions{ConfigDir: configDir, Name: "hello-world"}); err != nil {
		t.Fatalf("NewPlugin() error = %v", err)
	}

	result, err := WirePlugin(WireOptions{
		ConfigDir:   configDir,
		ImportPath:  "github.com/Ceinl/plums-user/plugins/hello-world",
		PackageName: "hello_world",
	})
	if err != nil {
		t.Fatalf("WirePlugin() error = %v", err)
	}
	if !result.Changed {
		t.Fatal("WirePlugin() Changed = false, want true")
	}
	data, err := os.ReadFile(filepath.Join(configDir, "config.go"))
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`hello_world "github.com/Ceinl/plums-user/plugins/hello-world"`,
		`hello_world.New(hello_world.Options{})`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config.go missing %q:\n%s", want, text)
		}
	}
	assertParseGo(t, filepath.Join(configDir, "config.go"))

	again, err := WirePlugin(WireOptions{
		ConfigDir:   configDir,
		ImportPath:  "github.com/Ceinl/plums-user/plugins/hello-world",
		PackageName: "hello_world",
	})
	if err != nil {
		t.Fatalf("second WirePlugin() error = %v", err)
	}
	if again.Changed {
		t.Fatal("second WirePlugin() Changed = true, want idempotent false")
	}
}

func TestListPluginsMarksWiredLocalPlugins(t *testing.T) {
	configDir := t.TempDir()
	if _, err := NewPlugin(PluginOptions{ConfigDir: configDir, Name: "hello-world"}); err != nil {
		t.Fatalf("NewPlugin() error = %v", err)
	}
	if _, err := WirePlugin(WireOptions{
		ConfigDir:   configDir,
		ImportPath:  "github.com/Ceinl/plums-user/plugins/hello-world",
		PackageName: "hello_world",
	}); err != nil {
		t.Fatalf("WirePlugin() error = %v", err)
	}

	plugins, err := ListPlugins(ConfigOptions{ConfigDir: configDir})
	if err != nil {
		t.Fatalf("ListPlugins() error = %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("plugins = %+v, want one", plugins)
	}
	if !plugins[0].Local || !plugins[0].Wired {
		t.Fatalf("plugin = %+v, want local wired", plugins[0])
	}
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

func TestPackageNameForImportPathHandlesMajorVersion(t *testing.T) {
	got := PackageNameForImportPath("github.com/me/plums-thing/v2@v2.0.0")
	if got != "plums_thing" {
		t.Fatalf("PackageNameForImportPath() = %q", got)
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
