package api

import (
	"flag"
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	cfgpkg "github.com/Ceinl/plums/config"
	"github.com/Ceinl/plums/internal/app"
	"github.com/Ceinl/plums/internal/builtincfg"
	"github.com/Ceinl/plums/internal/kernel"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	publiclayout "github.com/Ceinl/plums/plums/layout"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BackendProvider != "opencode" {
		t.Errorf("BackendProvider = %q, want opencode", cfg.BackendProvider)
	}
	if cfg.OpencodeServerURL != "" {
		t.Errorf("OpencodeServerURL = %q, want empty CLI override", cfg.OpencodeServerURL)
	}
	if cfg.ShowVersion || cfg.ShowDoctor || cfg.InitConfig {
		t.Error("action flags must default to false")
	}
}

func TestRegisterFlags(t *testing.T) {
	cfg := DefaultConfig()
	RegisterFlags(cfg)
	for _, name := range []string{"server-url", "provider", "init-config", "version", "doctor"} {
		if flag.Lookup(name) == nil {
			t.Errorf("flag %q not registered", name)
		}
	}
}

func TestRunShowVersion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ShowVersion = true
	cfg.Version = "1.2.3-test"
	if err := Run(cfg); err != nil {
		t.Errorf("Run with ShowVersion returned error: %v", err)
	}
}

func TestBackendRegistrationsFromRegistry(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryBackend, "opencode", capabilities.BackendRegistration{Name: "opencode"}, "core"); err != nil {
		t.Fatalf("register backend: %v", err)
	}
	if err := reg.Register(capabilities.RegistryBackend, "codex", capabilities.BackendRegistration{Name: "codex"}, "core"); err != nil {
		t.Fatalf("register backend: %v", err)
	}

	registrations, err := backendRegistrationsFromRegistry(reg)
	if err != nil {
		t.Fatalf("backendRegistrationsFromRegistry() error = %v", err)
	}
	if len(registrations) != 2 || registrations[0].Name != "opencode" || registrations[1].Name != "codex" {
		t.Fatalf("registrations = %+v", registrations)
	}
}

func TestBackendRegistrationsFromRegistryRejectsWrongType(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryBackend, "bad", "not a backend", "test"); err != nil {
		t.Fatalf("register backend: %v", err)
	}

	_, err := backendRegistrationsFromRegistry(reg)
	if err == nil {
		t.Fatal("expected wrong backend registry value to fail")
	}
}

func TestCommandsFromRegistry(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryCommand, "/new", capabilities.Command{Name: "/new"}, "core"); err != nil {
		t.Fatalf("register command: %v", err)
	}
	if err := reg.Register(capabilities.RegistryCommand, "/skills", capabilities.Command{Name: "/skills"}, "core"); err != nil {
		t.Fatalf("register command: %v", err)
	}

	commands, err := commandsFromRegistry(reg)
	if err != nil {
		t.Fatalf("commandsFromRegistry() error = %v", err)
	}
	if len(commands) != 2 || commands[0].Name != "/new" || commands[1].Name != "/skills" {
		t.Fatalf("commands = %+v", commands)
	}
}

func TestCommandsFromRegistryRejectsWrongType(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryCommand, "/bad", "not a command", "test"); err != nil {
		t.Fatalf("register command: %v", err)
	}

	_, err := commandsFromRegistry(reg)
	if err == nil {
		t.Fatal("expected wrong command registry value to fail")
	}
}

type testLayout struct {
	name string
	tree capabilities.Node
}

func (l testLayout) Name() string {
	return l.name
}

func (l testLayout) Tree() capabilities.Node {
	return l.tree
}

type testComponent struct {
	name string
}

func (c testComponent) Name() string {
	return c.name
}

func (c testComponent) Arrange(capabilities.Rect) {}

func (c testComponent) Render(capabilities.RenderCtx, capabilities.Surface) {}

type testOnMessage struct{}

func (testOnMessage) OnMessage(capabilities.Ctx, capabilities.Message) {}

func TestLayoutsFromRegistry(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryLayout, "chat", testLayout{name: "chat"}, "core"); err != nil {
		t.Fatalf("register layout: %v", err)
	}
	if err := reg.Register(capabilities.RegistryLayout, "split", testLayout{name: "split"}, "core"); err != nil {
		t.Fatalf("register layout: %v", err)
	}

	layouts, err := layoutsFromRegistry(reg, &app.RenderConfig{Layouts: map[string]app.LayoutNode{}})
	if err != nil {
		t.Fatalf("layoutsFromRegistry() error = %v", err)
	}
	if len(layouts) != 2 || layouts[0] != app.LayoutChat || layouts[1] != app.LayoutSplit {
		t.Fatalf("layouts = %+v", layouts)
	}
}

func TestLayoutsFromRegistryRejectsWrongType(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryLayout, "bad", "not a layout", "test"); err != nil {
		t.Fatalf("register layout: %v", err)
	}

	_, err := layoutsFromRegistry(reg, &app.RenderConfig{Layouts: map[string]app.LayoutNode{}})
	if err == nil {
		t.Fatal("expected wrong layout registry value to fail")
	}
}

func TestLayoutsFromRegistryInstallsLayoutTrees(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryLayout, "focus", testLayout{
		name: "focus",
		tree: publiclayout.Column(publiclayout.Chat(), publiclayout.InputBox().Height(7)),
	}, "plugin"); err != nil {
		t.Fatalf("register layout: %v", err)
	}
	renderConfig := &app.RenderConfig{Layouts: map[string]app.LayoutNode{}, Menu: []string{"chat"}}

	layouts, err := layoutsFromRegistry(reg, renderConfig)
	if err != nil {
		t.Fatalf("layoutsFromRegistry() error = %v", err)
	}
	if len(layouts) != 1 || layouts[0] != app.LayoutType("focus") {
		t.Fatalf("layouts = %+v, want focus", layouts)
	}
	node, ok := renderConfig.Layouts["focus"]
	if !ok {
		t.Fatal("focus layout tree was not installed")
	}
	if len(node.Children) != 2 || node.Children[1].Component != "input_box" {
		t.Fatalf("installed node = %+v", node)
	}
	if len(renderConfig.Menu) != 2 || renderConfig.Menu[1] != "focus" {
		t.Fatalf("menu = %+v, want chat, focus", renderConfig.Menu)
	}
}

func TestLayoutsFromRegistryInstallsHiddenLayoutsWithoutSelectingThem(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryLayout, "chat", testLayout{
		name: "chat",
		tree: publiclayout.Chat(),
	}, "core"); err != nil {
		t.Fatalf("register chat layout: %v", err)
	}
	if err := reg.Register(capabilities.RegistryLayout, "narrow_chat", publiclayout.Hidden(testLayout{
		name: "narrow_chat",
		tree: publiclayout.Chat(),
	}), "core"); err != nil {
		t.Fatalf("register hidden layout: %v", err)
	}
	renderConfig := &app.RenderConfig{Layouts: map[string]app.LayoutNode{}}

	layouts, err := layoutsFromRegistry(reg, renderConfig)
	if err != nil {
		t.Fatalf("layoutsFromRegistry() error = %v", err)
	}
	if len(layouts) != 1 || layouts[0] != app.LayoutChat {
		t.Fatalf("selectable layouts = %+v, want chat only", layouts)
	}
	if _, ok := renderConfig.Layouts["narrow_chat"]; !ok {
		t.Fatal("hidden fallback layout was not installed")
	}
	for _, name := range renderConfig.Menu {
		if name == "narrow_chat" {
			t.Fatalf("hidden layout leaked into menu: %+v", renderConfig.Menu)
		}
	}
}

func TestAppendLayoutIfMissing(t *testing.T) {
	layouts := appendLayoutIfMissing([]app.LayoutType{app.LayoutChat}, app.LayoutSplit)
	if len(layouts) != 2 || layouts[1] != app.LayoutSplit {
		t.Fatalf("layouts = %+v, want appended split", layouts)
	}
	layouts = appendLayoutIfMissing(layouts, app.LayoutSplit)
	if len(layouts) != 2 {
		t.Fatalf("duplicate layout appended: %+v", layouts)
	}
}

func TestComponentFactoriesFromRegistry(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	// Bundled components register under ui/components (not the old "core" owner).
	if err := reg.Register(capabilities.RegistryComponent, "status_bar", testComponent{name: "status_bar"}, "ui/components"); err != nil {
		t.Fatalf("register component: %v", err)
	}

	factories, err := componentFactoriesFromRegistry(reg)
	if err != nil {
		t.Fatalf("componentFactoriesFromRegistry() error = %v", err)
	}
	if len(factories) != 1 || factories["status_bar"] == nil {
		t.Fatalf("factories = %+v", factories)
	}
}

func TestComponentFactoriesFromRegistryWrapsPublicComponent(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryComponent, "custom", testComponent{name: "custom"}, "user"); err != nil {
		t.Fatalf("register component: %v", err)
	}

	factories, err := componentFactoriesFromRegistry(reg)
	if err != nil {
		t.Fatalf("componentFactoriesFromRegistry() error = %v", err)
	}
	if len(factories) != 1 || factories["custom"] == nil {
		t.Fatalf("factories = %+v", factories)
	}
}

func TestComponentFactoriesFromRegistryLetsUserOverrideBuiltInName(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	recorder := &testComponent{name: "status_bar"}
	if err := reg.Register(capabilities.RegistryComponent, "status_bar", recorder, "user"); err != nil {
		t.Fatalf("register component: %v", err)
	}

	factories, err := componentFactoriesFromRegistry(reg)
	if err != nil {
		t.Fatalf("componentFactoriesFromRegistry() error = %v", err)
	}
	component, err := factories["status_bar"](app.NewState(80, 24), app.LayoutNode{Component: "status_bar"})
	if err != nil {
		t.Fatalf("factory error = %v", err)
	}
	if _, ok := component.(*components.StatusBar); ok {
		t.Fatal("user status_bar override resolved to core StatusBar factory")
	}
}

func TestMergeOptsOverlaysExternalLastWins(t *testing.T) {
	base := cfgpkg.Opts{
		Backend:        "opencode",
		DefaultLayout:  "split",
		HideThinking:   cfgpkg.True,
		SplitLeftWidth: cfgpkg.Int(50),
		ClearHistory:   cfgpkg.False,
	}
	// External config pins backend + clear-history, flips hide_thinking, and
	// leaves layout / split width unset so they inherit the base.
	over := cfgpkg.Opts{
		Backend:      "codex",
		HideThinking: cfgpkg.False,
		ClearHistory: cfgpkg.True,
	}

	got := cfgpkg.MergeOpts(base, over).ToSettings(builtincfg.Defaults())
	if got.Backend != "codex" {
		t.Fatalf("Backend = %q, want codex", got.Backend)
	}
	if got.DefaultLayout != "split" {
		t.Fatalf("DefaultLayout = %q, want split fallback", got.DefaultLayout)
	}
	if got.HideThinking {
		t.Fatal("HideThinking fell back to true; external false must be respected")
	}
	if got.SplitLeftWidth != 50 {
		t.Fatalf("SplitLeftWidth = %d, want inherited 50", got.SplitLeftWidth)
	}
	if !got.ClearHistory {
		t.Fatal("ClearHistory = false, want external true")
	}
}

func TestMergeOptsUnsetTriStateInheritsBase(t *testing.T) {
	base := cfgpkg.Opts{HideThinking: cfgpkg.True, SplitLeftWidth: cfgpkg.Int(50)}
	// An all-zero overlay (everything unset) must not clobber base values.
	got := cfgpkg.MergeOpts(base, cfgpkg.Opts{}).ToSettings(builtincfg.Defaults())
	if !got.HideThinking {
		t.Fatal("unset HideThinking overlay clobbered base true")
	}
	if got.SplitLeftWidth != 50 {
		t.Fatalf("unset SplitLeftWidth overlay clobbered base: %d", got.SplitLeftWidth)
	}
}

func TestLayoutNameFromSettingsPrefersLayoutObject(t *testing.T) {
	got := layoutNameFromSettings(capabilities.Settings{Layout: testLayout{name: "zen"}, DefaultLayout: "split"})
	if got != "zen" {
		t.Fatalf("layout name = %q, want zen", got)
	}
}

func TestFormatDoctor(t *testing.T) {
	reg := kernel.NewRegistry(nil)
	if err := reg.Register(capabilities.RegistryBackend, "opencode", capabilities.BackendRegistration{Name: "opencode"}, "core"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(capabilities.RegistryCommand, "/new", capabilities.Command{Name: "/new"}, "core"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(capabilities.RegistryLayout, "split", testLayout{name: "split"}, "core"); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(capabilities.RegistryLayout, "narrow_split", publiclayout.Hidden(testLayout{name: "narrow_split"}), "core"); err != nil {
		t.Fatal(err)
	}
	loaded := &kernel.Loaded{
		Settings: capabilities.Settings{
			Backend:        "opencode",
			DefaultLayout:  "split",
			Theme:          capabilities.Theme{Name: "zen"},
			Keybinds:       []capabilities.Keybind{{Key: "ctrl+p", Do: "open_palette"}},
			Disable:        []capabilities.RegistryKey{{Kind: capabilities.RegistryCommand, Name: "/old"}},
			HideThinking:   true,
			SplitLeftWidth: 50,
		},
		Registry: reg,
		Hooks: kernel.Hooks{
			OnMessage: []capabilities.OnMessage{testOnMessage{}},
		},
	}

	got := formatDoctor(loaded)
	for _, want := range []string{
		"plums doctor",
		"settings.backend: opencode",
		"settings.default_layout: split",
		"settings.theme: zen",
		"settings.split_left_width: 50",
		"ctrl+p -> open_palette",
		"command./old",
		"on_message: 1",
		"opencode (core)",
		"/new (core)",
		"split (core)",
		"narrow_split (core, hidden)",
		"shadows:",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, got)
		}
	}
}
