package kernel

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	cfgpkg "github.com/Ceinl/plums/config"
)

type testPlugin struct {
	name     string
	commands []capabilities.Command
	settings capabilities.Settings
}

func (p *testPlugin) Name() string {
	return p.name
}

func (p *testPlugin) Init(host capabilities.Host, _ any) error {
	p.settings = host.Settings()
	host.Log("init %s", p.name)
	return nil
}

func (p *testPlugin) Commands() []capabilities.Command {
	return p.commands
}

func TestLoadUsesLastWinsOrder(t *testing.T) {
	core := &testPlugin{name: "core", commands: []capabilities.Command{{Name: "/open", Detail: "core"}}}
	userPlugin := &testPlugin{name: "user-plugin", commands: []capabilities.Command{{Name: "/open", Detail: "plugin"}}}
	// The external config's own commands now ship as a plugin too (last in slice
	// order, so it wins).
	configPlugin := &testPlugin{name: "config", commands: []capabilities.Command{{Name: "/open", Detail: "config"}}}
	cfg := cfgpkg.Config{
		Opts:    cfgpkg.Opts{Backend: "opencode"},
		Plugins: []cfgpkg.Plugin{{Self: userPlugin}, {Self: configPlugin}},
	}
	var logs []string
	loaded, err := Load(cfg, LoadOptions{
		Defaults: []cfgpkg.Plugin{{Self: core}},
		Settings: &capabilities.Settings{Backend: "opencode"},
		Logf: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	entry, ok := loaded.Registry.Entry(capabilities.RegistryCommand, "/open")
	if !ok {
		t.Fatal("expected /open command")
	}
	command := entry.Value.(capabilities.Command)
	if command.Detail != "config" {
		t.Fatalf("resolved command Detail = %q, want config", command.Detail)
	}
	if entry.Owner != "config" {
		t.Fatalf("resolved command owner = %q, want config", entry.Owner)
	}
	shadows := loaded.Registry.Shadows()
	if len(shadows) != 2 {
		t.Fatalf("shadow count = %d, want 2", len(shadows))
	}
	if shadows[0].PreviousOwner != "core" || shadows[0].NewOwner != "user-plugin" {
		t.Fatalf("first shadow = %+v", shadows[0])
	}
	if shadows[1].PreviousOwner != "user-plugin" || shadows[1].NewOwner != "config" {
		t.Fatalf("second shadow = %+v", shadows[1])
	}
	if !strings.Contains(strings.Join(logs, "\n"), "command./open") {
		t.Fatalf("logs do not include shadow entry: %v", logs)
	}
	if userPlugin.settings.Backend != "opencode" {
		t.Fatalf("plugin settings Backend = %q, want opencode", userPlugin.settings.Backend)
	}
}

func TestLoadAppliesDisableAfterOverrides(t *testing.T) {
	configPlugin := &testPlugin{name: "config", commands: []capabilities.Command{{Name: "/open", Detail: "config"}}}
	cfg := cfgpkg.Config{
		Plugins: []cfgpkg.Plugin{{Self: configPlugin}},
	}
	settings := capabilities.Settings{
		Disable: []capabilities.RegistryKey{{Kind: capabilities.RegistryCommand, Name: "/open"}},
	}
	loaded, err := Load(cfg, LoadOptions{Settings: &settings})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := loaded.Registry.Entry(capabilities.RegistryCommand, "/open"); ok {
		t.Fatal("disabled command still registered")
	}
}

func TestLoadSettingsOverrideKeepsPluginCapabilities(t *testing.T) {
	configPlugin := &testPlugin{name: "config", commands: []capabilities.Command{
		{Name: "/from-config", Detail: "config capability"},
	}}
	cfg := cfgpkg.Config{
		Opts:    cfgpkg.Opts{Backend: "config"},
		Plugins: []cfgpkg.Plugin{{Self: configPlugin}},
	}
	override := capabilities.Settings{Backend: "override"}

	loaded, err := Load(cfg, LoadOptions{Settings: &override})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Settings.Backend != "override" {
		t.Fatalf("loaded backend = %q, want override", loaded.Settings.Backend)
	}
	if _, ok := loaded.Registry.Entry(capabilities.RegistryCommand, "/from-config"); !ok {
		t.Fatal("config command capability was not activated")
	}
}

type completionPlugin struct {
	source capabilities.CompletionSource
}

func (completionPlugin) Name() string { return "completion-plugin" }

func (p completionPlugin) Init(host capabilities.Host, _ any) error {
	host.Services().Completion().Register(p.source)
	return nil
}

type fakeSource struct{ trigger rune }

func (s fakeSource) Trigger() rune                            { return s.trigger }
func (fakeSource) Candidates(string) []capabilities.Candidate { return nil }

func TestLoadCollectsCompletionSourcesFromHostServices(t *testing.T) {
	plugin := completionPlugin{source: fakeSource{trigger: '@'}}
	loaded, err := Load(cfgpkg.Config{Plugins: []cfgpkg.Plugin{{Self: plugin}}}, LoadOptions{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Completion == nil {
		t.Fatal("loaded.Completion is nil")
	}
	sources := loaded.Completion.Sources()
	if len(sources) != 1 || sources[0].Trigger() != '@' {
		t.Fatalf("registered sources = %+v", sources)
	}
}
