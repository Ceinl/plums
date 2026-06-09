package api

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Ceinl/plums/internal/app"
	"github.com/Ceinl/plums/internal/core"
	"github.com/Ceinl/plums/internal/core/adapter"
	"github.com/Ceinl/plums/internal/core/provider/codex"
	"github.com/Ceinl/plums/internal/core/provider/opencode"
	"github.com/Ceinl/plums/internal/debuglog"
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

	UseGlobalConfig bool
	UseLocalConfig  bool
	InitConfig      bool
	InitLocalConfig bool
	ShowVersion     bool
}

// DefaultConfig returns a Config populated with built-in defaults.
func DefaultConfig() *Config {
	defs := adapter.NewDefaultConfig()
	return &Config{
		Version:           defs.AppVersion,
		Commit:            "unknown",
		BuildDate:         "unknown",
		OpencodeServerURL: defs.DefaultBaseURL,
		BackendProvider:   "opencode",
		ClipboardCommand:  defs.ClipboardCommand,
	}
}

// RegisterFlags binds CLI flags to the supplied Config.
func RegisterFlags(cfg *Config) {
	flag.StringVar(&cfg.OpencodeServerURL, "server-url", cfg.OpencodeServerURL, "opencode server URL")
	flag.StringVar(&cfg.BackendProvider, "provider", cfg.BackendProvider, "backend provider: opencode or codex")
	flag.BoolVar(&cfg.UseGlobalConfig, "config-global", false, "use global plums layout config")
	flag.BoolVar(&cfg.UseGlobalConfig, "cg", false, "use global plums layout config")
	flag.BoolVar(&cfg.UseLocalConfig, "config-local", false, "use local plums layout config")
	flag.BoolVar(&cfg.UseLocalConfig, "cl", false, "use local plums layout config")
	flag.BoolVar(&cfg.InitConfig, "init-config", false, "create default config files in ~/.config/plums/config and exit")
	flag.BoolVar(&cfg.InitLocalConfig, "init-config-local", false, "create default config files in ./.agents/plums/config and exit")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "show version metadata and exit")
}

// Run wires all dependencies together and starts the application event loop.
func Run(cfg *Config) error {
	defer debuglog.Close()

	if cfg.ShowVersion {
		meta := app.LoadVersionMetadata(cfg.Version, cfg.Commit, cfg.BuildDate)
		fmt.Print(app.FormatVersionMetadata(meta))
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
	if cfg.InitLocalConfig {
		path, err := app.InitLocalConfigFiles()
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "created local plums config in %s\n", path)
		return nil
	}

	configPath, err := app.ResolveConfigPath(cfg.UseGlobalConfig, cfg.UseLocalConfig)
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys := keyboard.Listen(ctx)

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	opencodeConfigPath := app.ResolveOpencodeConfigPath(configPath)
	fallbackURL := adapter.DefaultBaseURLForDir(wd)
	serverURL, err := app.LoadOpencodeServerURL(opencodeConfigPath, fallbackURL)
	if err != nil {
		return fmt.Errorf("failed to load opencode server URL: %w", err)
	}
	backendProvider, err := app.LoadBackendProvider(opencodeConfigPath, cfg.BackendProvider)
	if err != nil {
		return fmt.Errorf("failed to load backend provider: %w", err)
	}
	defaultLayout, _ := app.LoadDefaultLayout(opencodeConfigPath, "split")
	hideThinking := app.LoadHideThinking(opencodeConfigPath, true)
	leftWidth := app.LoadSplitLeftWidth(opencodeConfigPath, 50)
	debuglog.Printf("config: opencode server URL %s", serverURL)
	debuglog.Printf("config: backend provider %s", backendProvider)
	debuglog.Printf("config: default_layout %s", defaultLayout)
	debuglog.Printf("config: hide_thinking %t", hideThinking)
	debuglog.Printf("config: split.left_width %d", leftWidth)
	opencodeBackend := opencode.NewBackend(serverURL)
	codexBackend := codex.NewBackend()

	registry := core.NewAgentRegistry()
	defs := adapter.NewDefaultConfig()

	runCfg := app.RunConfig{
		OpencodeServerURL:    serverURL,
		BackendProvider:      backendProvider,
		ClipboardCommand:     cfg.ClipboardCommand,
		SpinnerInterval:      defs.SpinnerInterval,
		HealthTimeout:        defs.HealthTimeout,
		QuestionReplyTimeout: defs.QuestionReplyTimeout,
		RecentModelTimeout:   defs.RecentModelTimeout,
		ListTimeout:          defs.ListTimeout,
		WorkingDirectory:     wd,
		ConfigTomlPath:       opencodeConfigPath,
		DefaultLayout:        defaultLayout,
		HideThinking:         hideThinking,
		SplitLeftWidth:       leftWidth,
	}

	deps := app.Deps{
		Terminal:      t,
		Keyboard:      keys,
		RenderConfig:  renderConfig,
		CommandConfig: commandConfig,
		Registry:      registry,
		Backends: []app.BackendRuntime{
			{ID: "opencode", Name: "Opencode", Backend: opencodeBackend, Startup: opencodeStartup(wd, runCfg.HealthTimeout)},
			{ID: "codex", Name: "Codex", Backend: codexBackend, Startup: codexStartup(wd, codexBackend)},
		},
	}

	server, err := app.Run(ctx, deps, runCfg)
	if server != nil {
		server.Stop()
	}
	return err
}

func opencodeStartup(wd string, healthTimeout time.Duration) func(context.Context, adapter.Backend, chan<- app.StartupResult) {
	return func(startCtx context.Context, b adapter.Backend, out chan<- app.StartupResult) {
		var serverProc *opencode.ServerProcess
		debuglog.Printf("startup: checking opencode health")
		if err := b.Health(startCtx); err != nil {
			debuglog.Printf("startup: health check failed: %v", err)
			proc, err := opencode.StartServer(startCtx, b.BaseURL(), wd)
			if err != nil {
				debuglog.Printf("startup: start server failed: %v", err)
				out <- app.StartupResult{Err: fmt.Errorf("failed to start opencode server: %w", err)}
				return
			}
			serverProc = proc
			debuglog.Printf("startup: started opencode server process")
			if err := opencode.WaitForHealthOrExit(startCtx, b, serverProc, healthTimeout); err != nil {
				debuglog.Printf("startup: wait for health failed: %v", err)
				serverProc.Stop()
				out <- app.StartupResult{Err: fmt.Errorf("failed to start opencode server: %w", err)}
				return
			}
		} else {
			debuglog.Printf("startup: existing opencode server is healthy")
		}

		debuglog.Printf("startup: opencode ready (session deferred)")
		out <- app.StartupResult{Server: serverProc}
	}
}

func codexStartup(wd string, backend adapter.Backend) func(context.Context, adapter.Backend, chan<- app.StartupResult) {
	return func(startCtx context.Context, b adapter.Backend, out chan<- app.StartupResult) {
		debuglog.Printf("startup: codex ready (session deferred)")
		client, ok := b.(interface {
			ServerProcess() interface {
				Stop()
				Done() <-chan struct{}
			}
		})
		if !ok {
			out <- app.StartupResult{Err: fmt.Errorf("codex backend does not expose ServerProcess")}
			return
		}
		out <- app.StartupResult{Server: client.ServerProcess()}
	}
}
