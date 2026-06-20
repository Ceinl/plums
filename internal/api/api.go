package api

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Ceinl/plums/capabilities"
	cfgpkg "github.com/Ceinl/plums/config"
	"github.com/Ceinl/plums/internal/app"
	"github.com/Ceinl/plums/internal/builtincfg"
	"github.com/Ceinl/plums/internal/core"
	opencodebackend "github.com/Ceinl/plums/internal/core/backend/opencode"
	"github.com/Ceinl/plums/internal/debuglog"
	"github.com/Ceinl/plums/internal/kernel"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui"
)

// Config holds every runtime tunable. All fields have sensible defaults.
type Config struct {
	Version   string
	Commit    string
	BuildDate string

	OpencodeServerURL string
	BackendProvider   string
	ClipboardCommand  string

	InitConfig   bool
	ClearHistory bool
	ShowVersion  bool
	ShowDoctor   bool
}

type runtimeDefaults struct {
	AppVersion           string
	ClipboardCommand     string
	SpinnerInterval      time.Duration
	HealthTimeout        time.Duration
	QuestionReplyTimeout time.Duration
	RecentModelTimeout   time.Duration
	ListTimeout          time.Duration
}

func defaultRuntimeDefaults() runtimeDefaults {
	return runtimeDefaults{
		AppVersion:           "0.1.0-dev",
		ClipboardCommand:     "pbcopy",
		SpinnerInterval:      80 * time.Millisecond,
		HealthTimeout:        10 * time.Second,
		QuestionReplyTimeout: 5 * time.Second,
		RecentModelTimeout:   2 * time.Second,
		ListTimeout:          3 * time.Second,
	}
}

type resolvedKernelConfig struct {
	Config             cfgpkg.Config
	Settings           capabilities.Settings
	OpencodeConfigPath string
}

// resolveKernelConfig builds the effective config by merging, last-wins:
//
//	built-in Default Config  ←  TOML/env-derived overlay  ←  external user config
//
// then projects the merged Opts into the runtime-facing capabilities.Settings.
func resolveKernelConfig(cfg *Config, configPath, wd string) (*resolvedKernelConfig, error) {
	opencodeConfigPath := app.ResolveOpencodeConfigPath(configPath)
	serverURL, err := app.LoadOpencodeServerURL(opencodeConfigPath, opencodebackend.DefaultBaseURLForDir(wd))
	if err != nil {
		return nil, fmt.Errorf("failed to load opencode server URL: %w", err)
	}
	backendProvider, err := app.LoadBackendProvider(opencodeConfigPath, cfg.BackendProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to load backend provider: %w", err)
	}
	defaultLayout, _ := app.LoadDefaultLayout(opencodeConfigPath, "chat")
	defs := defaultRuntimeDefaults()

	base := builtincfg.Config(builtincfg.RuntimeParams{
		WorkingDirectory:  wd,
		OpencodeServerURL: serverURL,
		HealthTimeout:     defs.HealthTimeout,
	})

	// TOML/env-derived overlay — today's file/flag-driven settings, expressed as
	// Opts so they slot into the same merge as everything else.
	overlay := cfgpkg.Config{Opts: cfgpkg.Opts{
		Backend:           cfgpkg.Pref(backendProvider),
		DefaultLayout:     cfgpkg.Pref(defaultLayout),
		HideThinking:      boolPref(app.LoadHideThinking(opencodeConfigPath, true)),
		SplitLeftWidth:    cfgpkg.Int(app.LoadSplitLeftWidth(opencodeConfigPath, 50)),
		ClearHistory:      boolPref(cfg.ClearHistory || app.LoadClearHistory(opencodeConfigPath, false)),
		OpencodeServerURL: cfgpkg.Pref(serverURL),
	}}

	merged := cfgpkg.Merge(base, overlay)
	if external, ok := cfgpkg.Registered(); ok {
		merged = cfgpkg.Merge(merged, external())
	}
	if cfg != nil && cfg.ClearHistory {
		merged.Opts.ClearHistory = cfgpkg.True
	}

	settings := merged.Opts.ToSettings(builtincfg.Defaults())
	return &resolvedKernelConfig{
		Config:             merged,
		Settings:           settings,
		OpencodeConfigPath: opencodeConfigPath,
	}, nil
}

func boolPref(v bool) cfgpkg.Bool {
	if v {
		return cfgpkg.True
	}
	return cfgpkg.False
}

func layoutNameFromSettings(settings capabilities.Settings) string {
	if settings.Layout != nil {
		return settings.Layout.Name()
	}
	return settings.DefaultLayout
}

// DefaultConfig returns a Config populated with built-in defaults.
func DefaultConfig() *Config {
	defs := defaultRuntimeDefaults()
	return &Config{
		Version:           defs.AppVersion,
		Commit:            "unknown",
		BuildDate:         "unknown",
		OpencodeServerURL: opencodebackend.DefaultBaseURL,
		BackendProvider:   "opencode",
		ClipboardCommand:  defs.ClipboardCommand,
	}
}

// RegisterFlags binds CLI flags to the supplied Config.
func RegisterFlags(cfg *Config) {
	flag.StringVar(&cfg.OpencodeServerURL, "server-url", cfg.OpencodeServerURL, "opencode server URL")
	flag.StringVar(&cfg.BackendProvider, "provider", cfg.BackendProvider, "backend provider: opencode, codex, claude, or claude-mirror")
	flag.BoolVar(&cfg.ClearHistory, "clear-history", false, "hide pre-existing backend sessions; show only sessions created in this run")
	flag.BoolVar(&cfg.ClearHistory, "ch", false, "hide pre-existing backend sessions; show only sessions created in this run")
	flag.BoolVar(&cfg.InitConfig, "init-config", false, "create default config files in ~/.config/plums/config and exit")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "show version metadata and exit")
	flag.BoolVar(&cfg.ShowDoctor, "doctor", false, "print resolved plums registry and exit")
}

// Run wires all dependencies together and starts the application event loop.
func Run(cfg *Config) error {
	defer debuglog.Close()

	if cfg.ShowVersion {
		meta := app.LoadVersionMetadata(cfg.Version, cfg.Commit, cfg.BuildDate)
		fmt.Print(app.FormatVersionMetadata(meta))
		return nil
	}
	if cfg.ShowDoctor {
		loaded, err := loadKernelForConfig(cfg)
		if err != nil {
			return err
		}
		fmt.Print(formatDoctor(loaded))
		return nil
	}

	if cfg.InitConfig {
		path, err := app.InitGlobalConfig()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "created plums config in %s\n", path)
		return nil
	}

	configPath, err := app.ResolveConfigPath()
	if err != nil {
		return err
	}
	renderConfig, err := app.LoadRenderConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load layout config: %w", err)
	}
	commandConfigPath, err := app.ResolveCommandsConfigPath(configPath)
	if err != nil {
		return err
	}
	commandConfig, err := app.LoadCommandConfig(commandConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load command config: %w", err)
	}
	if configPath != "" {
		debuglog.Printf("config: using %s", configPath)
	} else {
		debuglog.Printf("config: using built-in layout config")
	}
	if commandConfigPath != "" {
		debuglog.Printf("config: using %s", commandConfigPath)
	} else {
		debuglog.Printf("config: using built-in command config")
	}

	t := ui.NewTerminal(int(os.Stdin.Fd()))
	if err := t.Enter(); err != nil {
		return fmt.Errorf("failed to initialize terminal: %w", err)
	}
	defer t.Exit()

	// Inside tmux, Shift+Enter only reaches the app when extended-keys is on;
	// enable it for this run so split-layout submit works, then restore it.
	tmuxKeys := ui.EnableTmuxExtendedKeys()
	defer tmuxKeys.Restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys := keyboard.Listen(ctx)

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	resolved, err := resolveKernelConfig(cfg, configPath, wd)
	if err != nil {
		return err
	}
	settings := resolved.Settings
	defaultLayout := layoutNameFromSettings(settings)
	debuglog.Printf("config: opencode server URL %s", settings.OpencodeServerURL)
	debuglog.Printf("config: backend provider %s", settings.Backend)
	debuglog.Printf("config: default_layout %s", defaultLayout)
	debuglog.Printf("config: hide_thinking %t", settings.HideThinking)
	debuglog.Printf("config: split.left_width %d", settings.SplitLeftWidth)
	debuglog.Printf("config: clear_history %t", settings.ClearHistory)

	registry := core.NewAgentRegistry()
	defs := defaultRuntimeDefaults()

	runCfg := app.RunConfig{
		OpencodeServerURL:    settings.OpencodeServerURL,
		BackendProvider:      settings.Backend,
		ClipboardCommand:     cfg.ClipboardCommand,
		SpinnerInterval:      defs.SpinnerInterval,
		HealthTimeout:        defs.HealthTimeout,
		QuestionReplyTimeout: defs.QuestionReplyTimeout,
		RecentModelTimeout:   defs.RecentModelTimeout,
		ListTimeout:          defs.ListTimeout,
		WorkingDirectory:     wd,
		ConfigTomlPath:       resolved.OpencodeConfigPath,
		DefaultLayout:        defaultLayout,
		Theme:                settings.Theme,
		HideThinking:         settings.HideThinking,
		SplitLeftWidth:       settings.SplitLeftWidth,
		ClearHistory:         settings.ClearHistory,
	}

	loaded, err := kernel.Load(resolved.Config, kernel.LoadOptions{
		Settings: &settings,
		Logf:     debuglog.Printf,
	})
	if err != nil {
		return fmt.Errorf("failed to load kernel config: %w", err)
	}
	backendRegistrations, err := backendRegistrationsFromRegistry(loaded.Registry)
	if err != nil {
		return err
	}
	backendRuntimes := app.BackendRuntimesFromRegistrations(backendRegistrations)
	if len(backendRuntimes) == 0 {
		return fmt.Errorf("no backend registrations loaded")
	}
	commands, err := commandsFromRegistry(loaded.Registry)
	if err != nil {
		return err
	}
	layouts, err := layoutsFromRegistry(loaded.Registry, renderConfig)
	if err != nil {
		return err
	}
	if loaded.Settings.Layout != nil {
		layoutType, err := app.InstallPublicLayout(renderConfig, loaded.Settings.Layout)
		if err != nil {
			return err
		}
		layouts = appendLayoutIfMissing(layouts, layoutType)
	}
	components, err := componentFactoriesFromRegistry(loaded.Registry)
	if err != nil {
		return err
	}
	runCfg.BackendProvider = loaded.Settings.Backend

	deps := app.Deps{
		Terminal:      t,
		Keyboard:      keys,
		RenderConfig:  renderConfig,
		Layouts:       layouts,
		CommandConfig: commandConfig,
		Commands:      commands,
		Keybinds:      loaded.Settings.Keybinds,
		Components:    components,
		Hooks: app.Hooks{
			OnMessage:      loaded.Hooks.OnMessage,
			OnSessionStart: loaded.Hooks.OnSessionStart,
			OnToolCall:     loaded.Hooks.OnToolCall,
		},
		Registry: registry,
		Backends: backendRuntimes,
	}
	if loaded.Completion != nil {
		deps.CompletionSources = loaded.Completion.Sources()
	}

	server, err := app.Run(ctx, deps, runCfg)
	if server != nil {
		server.Stop()
	}
	return err
}

func backendRegistrationsFromRegistry(reg *kernel.Registry) ([]capabilities.BackendRegistration, error) {
	if reg == nil {
		return nil, fmt.Errorf("nil kernel registry")
	}
	entries := reg.EntriesInRegistrationOrder(capabilities.RegistryBackend)
	registrations := make([]capabilities.BackendRegistration, 0, len(entries))
	for _, entry := range entries {
		registration, ok := entry.Value.(capabilities.BackendRegistration)
		if !ok {
			return nil, fmt.Errorf("registry backend %q from %s has type %T", entry.Name, entry.Owner, entry.Value)
		}
		registrations = append(registrations, registration)
	}
	return registrations, nil
}

func loadKernelForConfig(cfg *Config) (*kernel.Loaded, error) {
	configPath, err := app.ResolveConfigPath()
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}
	resolved, err := resolveKernelConfig(cfg, configPath, wd)
	if err != nil {
		return nil, err
	}
	return kernel.Load(resolved.Config, kernel.LoadOptions{
		Settings: &resolved.Settings,
		Logf:     debuglog.Printf,
	})
}

func formatDoctor(loaded *kernel.Loaded) string {
	if loaded == nil || loaded.Registry == nil {
		return "plums doctor\n\nregistry: unavailable\n"
	}
	var b strings.Builder
	b.WriteString("plums doctor\n\n")
	themeName := loaded.Settings.Theme.Name
	if themeName == "" {
		themeName = "default"
	}
	fmt.Fprintf(&b, "settings.backend: %s\n", loaded.Settings.Backend)
	fmt.Fprintf(&b, "settings.default_layout: %s\n", layoutNameFromSettings(loaded.Settings))
	fmt.Fprintf(&b, "settings.theme: %s\n", themeName)
	fmt.Fprintf(&b, "settings.hide_thinking: %t\n", loaded.Settings.HideThinking)
	fmt.Fprintf(&b, "settings.split_left_width: %d\n", loaded.Settings.SplitLeftWidth)
	fmt.Fprintf(&b, "settings.clear_history: %t\n", loaded.Settings.ClearHistory)
	fmt.Fprintf(&b, "settings.opencode_server_url: %s\n", loaded.Settings.OpencodeServerURL)
	writeKeybinds(&b, loaded.Settings.Keybinds)
	writeDisabled(&b, loaded.Settings.Disable)
	fmt.Fprintf(&b, "\nhooks:\n")
	fmt.Fprintf(&b, "  on_message: %d\n", len(loaded.Hooks.OnMessage))
	fmt.Fprintf(&b, "  on_session_start: %d\n", len(loaded.Hooks.OnSessionStart))
	fmt.Fprintf(&b, "  on_tool_call: %d\n", len(loaded.Hooks.OnToolCall))
	writeRegistryKind(&b, loaded.Registry, capabilities.RegistryBackend, "backends")
	writeRegistryKind(&b, loaded.Registry, capabilities.RegistryCommand, "commands")
	writeRegistryKind(&b, loaded.Registry, capabilities.RegistryComponent, "components")
	writeRegistryKind(&b, loaded.Registry, capabilities.RegistryLayout, "layouts")
	shadows := loaded.Registry.Shadows()
	b.WriteString("\nshadows:\n")
	if len(shadows) == 0 {
		b.WriteString("  none\n")
		return b.String()
	}
	for _, shadow := range shadows {
		fmt.Fprintf(&b, "  %s.%s: %s -> %s\n", shadow.Kind, shadow.Name, shadow.PreviousOwner, shadow.NewOwner)
	}
	return b.String()
}

func writeKeybinds(b *strings.Builder, keybinds []capabilities.Keybind) {
	b.WriteString("\nkeybinds:\n")
	if len(keybinds) == 0 {
		b.WriteString("  none\n")
		return
	}
	for _, keybind := range keybinds {
		fmt.Fprintf(b, "  %s -> %s\n", keybind.Key, keybind.Do)
	}
}

func writeDisabled(b *strings.Builder, keys []capabilities.RegistryKey) {
	b.WriteString("\ndisabled:\n")
	if len(keys) == 0 {
		b.WriteString("  none\n")
		return
	}
	for _, key := range keys {
		fmt.Fprintf(b, "  %s.%s\n", key.Kind, key.Name)
	}
}

func writeRegistryKind(b *strings.Builder, reg *kernel.Registry, kind capabilities.RegistryKind, title string) {
	entries := reg.EntriesInRegistrationOrder(kind)
	fmt.Fprintf(b, "\n%s:\n", title)
	if len(entries) == 0 {
		b.WriteString("  none\n")
		return
	}
	for _, entry := range entries {
		if kind == capabilities.RegistryLayout {
			if layout, ok := entry.Value.(capabilities.Layout); ok && !app.PublicLayoutSelectable(layout) {
				fmt.Fprintf(b, "  %s (%s, hidden)\n", entry.Name, entry.Owner)
				continue
			}
		}
		fmt.Fprintf(b, "  %s (%s)\n", entry.Name, entry.Owner)
	}
}

func commandsFromRegistry(reg *kernel.Registry) ([]capabilities.Command, error) {
	if reg == nil {
		return nil, fmt.Errorf("nil kernel registry")
	}
	entries := reg.EntriesInRegistrationOrder(capabilities.RegistryCommand)
	commands := make([]capabilities.Command, 0, len(entries))
	for _, entry := range entries {
		command, ok := entry.Value.(capabilities.Command)
		if !ok {
			return nil, fmt.Errorf("registry command %q from %s has type %T", entry.Name, entry.Owner, entry.Value)
		}
		commands = append(commands, command)
	}
	return commands, nil
}

func layoutsFromRegistry(reg *kernel.Registry, renderConfig *app.RenderConfig) ([]app.LayoutType, error) {
	if reg == nil {
		return nil, fmt.Errorf("nil kernel registry")
	}
	entries := reg.EntriesInRegistrationOrder(capabilities.RegistryLayout)
	layouts := make([]app.LayoutType, 0, len(entries))
	for _, entry := range entries {
		layout, ok := entry.Value.(capabilities.Layout)
		if !ok {
			return nil, fmt.Errorf("registry layout %q from %s has type %T", entry.Name, entry.Owner, entry.Value)
		}
		if layout.Tree() != nil {
			layoutType, err := app.InstallPublicLayout(renderConfig, layout)
			if err != nil {
				return nil, err
			}
			if app.PublicLayoutSelectable(layout) {
				layouts = appendLayoutIfMissing(layouts, layoutType)
			}
			continue
		}
		if app.PublicLayoutSelectable(layout) {
			layouts = appendLayoutIfMissing(layouts, app.LayoutType(layout.Name()))
		}
	}
	return layouts, nil
}

func appendLayoutIfMissing(layouts []app.LayoutType, layout app.LayoutType) []app.LayoutType {
	if layout == "" {
		return layouts
	}
	for _, existing := range layouts {
		if existing == layout {
			return layouts
		}
	}
	return append(layouts, layout)
}

func componentFactoriesFromRegistry(reg *kernel.Registry) (map[string]app.ComponentFactory, error) {
	if reg == nil {
		return nil, fmt.Errorf("nil kernel registry")
	}
	entries := reg.EntriesInRegistrationOrder(capabilities.RegistryComponent)
	factories := make(map[string]app.ComponentFactory, len(entries))
	for _, entry := range entries {
		component, ok := entry.Value.(capabilities.Component)
		if !ok {
			return nil, fmt.Errorf("registry component %q from %s has type %T", entry.Name, entry.Owner, entry.Value)
		}
		// A bundled component that wraps an internal factory exposes it via
		// ComponentFactoryProvider; everything else renders through the public
		// Component/Surface path. Ownership no longer gates this.
		factory := app.ComponentFactoryForPublic(component)
		if provider, ok := component.(app.ComponentFactoryProvider); ok {
			factory = provider.AppComponentFactory()
		}
		factories[component.Name()] = factory
	}
	return factories, nil
}
