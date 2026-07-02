package builtin

import (
	"fmt"
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
	)
	return plugins
}

// BackendPlugins returns one plugin per bundled backend, named backend/<name>.
func BackendPlugins(options CoreOptions) []cfgpkg.Plugin {
	return []cfgpkg.Plugin{
		{
			Self: &backendPlugin{
				name: "backend/opencode",
				init: func(opts any) (capabilities.BackendRegistration, error) {
					opencodeOpts, ok := opts.(opencodebackend.Options)
					if !ok {
						return capabilities.BackendRegistration{}, fmt.Errorf("backend/opencode opts have type %T, want opencode.Options", opts)
					}
					return opencodebackend.Registration(opencodeOpts), nil
				},
			},
			Opts: opencodebackend.Options{
				WorkingDirectory: options.WorkingDirectory,
				ServerURL:        options.OpencodeServerURL,
				HealthTimeout:    options.HealthTimeout,
			},
		},
		{Self: &backendPlugin{name: "backend/codex", registration: codexbackend.Registration()}},
		{Self: &backendPlugin{name: "backend/claude", registration: claudebackend.Registration()}},
		{Self: &backendPlugin{name: "backend/claude-mirror", registration: mirrorbackend.Registration()}},
	}
}

type backendPlugin struct {
	name         string
	registration capabilities.BackendRegistration
	init         func(any) (capabilities.BackendRegistration, error)
}

func (p *backendPlugin) Name() string { return p.name }
func (p *backendPlugin) Init(_ capabilities.Host, opts any) error {
	if p.init == nil {
		return nil
	}
	registration, err := p.init(opts)
	if err != nil {
		return err
	}
	p.registration = registration
	return nil
}
func (p *backendPlugin) Backends() []capabilities.BackendRegistration {
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
