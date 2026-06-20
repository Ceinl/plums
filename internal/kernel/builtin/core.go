package builtin

import (
	"time"

	"github.com/Ceinl/plums/capabilities"
	cfgpkg "github.com/Ceinl/plums/config"
	claudebackend "github.com/Ceinl/plums/internal/core/backend/claudecode"
	mirrorbackend "github.com/Ceinl/plums/internal/core/backend/claudemirror"
	codexbackend "github.com/Ceinl/plums/internal/core/backend/codex"
	opencodebackend "github.com/Ceinl/plums/internal/core/backend/opencode"
)

type CoreOptions struct {
	WorkingDirectory  string
	OpencodeServerURL string
	HealthTimeout     time.Duration
}

// DefaultPlugins returns the bundled default plugin set as config.Plugin values,
// lowest priority, in load order: one plugin per backend, then the grouped UI
// plugins (components, commands). Layouts are NOT here — they ship as a public
// layout plugin wired by the Default Config (see internal/builtincfg). Each
// plugin is a distinct registry owner, so a user config can shadow or Disable
// any single one (e.g. drop backend/codex) without re-providing the rest.
func DefaultPlugins(options CoreOptions) []cfgpkg.Plugin {
	plugins := BackendPlugins(options)
	plugins = append(plugins,
		cfgpkg.Plugin{Self: componentsPlugin{components: DefaultComponents()}},
		cfgpkg.Plugin{Self: commandsPlugin{commands: DefaultCommands()}},
	)
	return plugins
}

// BackendPlugins returns one plugin per bundled backend, named backend/<name>.
func BackendPlugins(options CoreOptions) []cfgpkg.Plugin {
	registrations := BackendRegistrations(options)
	plugins := make([]cfgpkg.Plugin, 0, len(registrations))
	for _, registration := range registrations {
		plugins = append(plugins, cfgpkg.Plugin{Self: backendPlugin{
			name:         "backend/" + registration.Name,
			registration: registration,
		}})
	}
	return plugins
}

type backendPlugin struct {
	name         string
	registration capabilities.BackendRegistration
}

func (p backendPlugin) Name() string                      { return p.name }
func (p backendPlugin) Init(capabilities.Host, any) error { return nil }
func (p backendPlugin) Backends() []capabilities.BackendRegistration {
	return []capabilities.BackendRegistration{p.registration}
}

type componentsPlugin struct {
	components []capabilities.Component
}

func (componentsPlugin) Name() string                      { return "ui/components" }
func (componentsPlugin) Init(capabilities.Host, any) error { return nil }
func (p componentsPlugin) Components() []capabilities.Component {
	return append([]capabilities.Component(nil), p.components...)
}

type commandsPlugin struct {
	commands []capabilities.Command
}

func (commandsPlugin) Name() string                      { return "ui/commands" }
func (commandsPlugin) Init(capabilities.Host, any) error { return nil }
func (p commandsPlugin) Commands() []capabilities.Command {
	return append([]capabilities.Command(nil), p.commands...)
}

func DefaultCommands() []capabilities.Command {
	return []capabilities.Command{
		{Name: "/new", Detail: "Create a fresh opencode session"},
		{Name: "/command", Detail: "Open the command palette"},
		{Name: "/backend", Detail: "Switch backend provider"},
		{Name: "/skills", Detail: "Load an opencode skill"},
		{Name: "/sessions", Detail: "Open existing opencode sessions"},
		{Name: "/model", Detail: "Change the active model"},
	}
}

func BackendRegistrations(options CoreOptions) []capabilities.BackendRegistration {
	return []capabilities.BackendRegistration{
		opencodebackend.Registration(opencodebackend.Options{
			WorkingDirectory: options.WorkingDirectory,
			ServerURL:        options.OpencodeServerURL,
			HealthTimeout:    options.HealthTimeout,
		}),
		codexbackend.Registration(),
		claudebackend.Registration(),
		mirrorbackend.Registration(),
	}
}
