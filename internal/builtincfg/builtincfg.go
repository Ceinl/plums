// Package builtincfg is the App's built-in Default Config. It returns a
// config.Config using the exact same {Opts, Plugins} shape as an external user
// config, compiled into the binary at lowest priority. The external config is
// merged over this (config.Merge).
//
// This is the dogfooding floor: the app's own defaults are expressed through the
// public config surface, not a privileged internal path.
package builtincfg

import (
	"time"

	cfgpkg "github.com/Ceinl/plums/config"
	"github.com/Ceinl/plums/internal/kernel/builtin"
	backendcommands "github.com/Ceinl/plums/plugins/backend"
	"github.com/Ceinl/plums/plugins/commands"
	"github.com/Ceinl/plums/plugins/gitdiff"
	"github.com/Ceinl/plums/plugins/layouts"
	"github.com/Ceinl/plums/plugins/questions"
	"github.com/Ceinl/plums/plugins/sessions"
	"github.com/Ceinl/plums/plugins/skills"
)

// RuntimeParams carries the launch-time values the bundled backend plugins need
// (working directory, resolved opencode server URL, health timeout). They are
// runtime-resolved, so they are passed in rather than baked into the Default
// Config's source.
type RuntimeParams struct {
	WorkingDirectory  string
	OpencodeServerURL string
	HealthTimeout     time.Duration
}

// Defaults mirrors today's hardcoded runtime defaults for the Opts that ship in
// the built-in Default Config.
func Defaults() cfgpkg.SettingsDefaults {
	return cfgpkg.SettingsDefaults{
		HideThinking:   true,
		SplitLeftWidth: 50,
		OutputPercent:  0,
		ClearHistory:   false,
	}
}

// Opts returns the built-in Default Config's Opts: today's settings values,
// expressed in the sentinel types.
func Opts() cfgpkg.Opts {
	return cfgpkg.Opts{
		Backend:          "opencode",
		DefaultLayout:    "chat",
		Theme:            cfgpkg.Theme{Name: "default"},
		Keybinds:         DefaultKeybinds(),
		ClipboardCommand: "pbcopy",
		HideThinking:     cfgpkg.True,
		SplitLeftWidth:   cfgpkg.Int(50),
	}
}

// DefaultKeybinds is the stock keymap expressed as Default Config data. These
// bindings target registered command names; legacy aliases remain accepted only
// for existing user configs.
func DefaultKeybinds() []cfgpkg.Keybind {
	return []cfgpkg.Keybind{
		{Key: "ctrl+p", Do: "palette.open"},
		{Key: "ctrl+s", Do: "prompt.submit"},
	}
}

// Config returns the complete built-in Default Config: the default Opts plus the
// bundled plugin set (backends + UI components/commands + the public layouts
// plugin) as config.Plugin values.
func Config(params RuntimeParams) cfgpkg.Config {
	plugins := builtin.DefaultPlugins(builtin.CoreOptions{
		WorkingDirectory:  params.WorkingDirectory,
		OpencodeServerURL: params.OpencodeServerURL,
		HealthTimeout:     params.HealthTimeout,
	})
	plugins = append(plugins,
		cfgpkg.Plugin{Self: &layouts.Plugin{}},
		cfgpkg.Plugin{Self: &backendcommands.Plugin{}},
		cfgpkg.Plugin{Self: &sessions.Plugin{}},
		cfgpkg.Plugin{Self: &commands.Plugin{}},
		cfgpkg.Plugin{Self: &skills.Plugin{}},
		cfgpkg.Plugin{Self: &questions.Plugin{}},
		cfgpkg.Plugin{Self: &gitdiff.Plugin{}},
	)
	return cfgpkg.Config{
		Opts:    Opts(),
		Plugins: plugins,
	}
}
