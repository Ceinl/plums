package kernel

import (
	"fmt"

	"github.com/Ceinl/plums/capabilities"
	cfgpkg "github.com/Ceinl/plums/config"
)

type LoadOptions struct {
	// Defaults are the bundled Default Config plugins, activated first (lowest
	// priority) before the external config's plugins.
	Defaults []cfgpkg.Plugin
	// Settings is the resolved, merged Settings the host exposes to plugins. When
	// nil the loader projects it from cfg.Opts with zero SettingsDefaults.
	Settings *capabilities.Settings
	Logf     func(string, ...any)
}

type Loaded struct {
	Settings capabilities.Settings
	Registry *Registry
	Hooks    Hooks
	// Completion holds the completion sources plugins contributed via
	// Host.Services().Completion().Register at Init. Core consumes these (plus its
	// own built-in @file/slash sources) at runtime.
	Completion *CompletionRegistry
}

type Hooks struct {
	OnMessage      []capabilities.OnMessage
	OnSessionStart []capabilities.OnSessionStart
	OnToolCall     []capabilities.OnToolCall
}

func Load(cfg cfgpkg.Config, opts LoadOptions) (*Loaded, error) {
	loaded := &Loaded{
		Registry:   NewRegistry(opts.Logf),
		Completion: newCompletionRegistry(),
	}
	if opts.Settings != nil {
		loaded.Settings = *opts.Settings
	} else {
		loaded.Settings = cfg.Opts.ToSettings(cfgpkg.SettingsDefaults{})
	}
	host := host{settings: loaded.Settings, logf: opts.Logf, services: hostServices{completion: loaded.Completion}}

	for _, plugin := range opts.Defaults {
		if plugin.Self == nil {
			continue
		}
		if err := activatePlugin(plugin, host, loaded); err != nil {
			return nil, err
		}
	}
	for _, plugin := range cfg.Plugins {
		if plugin.Self == nil {
			continue
		}
		if err := activatePlugin(plugin, host, loaded); err != nil {
			return nil, err
		}
	}
	loaded.Registry.Disable(loaded.Settings.Disable)
	return loaded, nil
}

type host struct {
	settings capabilities.Settings
	logf     func(string, ...any)
	services capabilities.Services
}

func (h host) Settings() capabilities.Settings {
	return h.settings
}

func (h host) Log(format string, args ...any) {
	if h.logf != nil {
		h.logf(format, args...)
	}
}

func (h host) Services() capabilities.Services {
	return h.services
}

func activatePlugin(plugin cfgpkg.Plugin, host host, loaded *Loaded) error {
	owner := "plugin"
	if named, ok := plugin.Self.(interface{ Name() string }); ok {
		owner = named.Name()
		if owner == "" {
			return fmt.Errorf("kernel: plugin has empty name")
		}
	}
	if initer, ok := plugin.Self.(capabilities.Plugin); ok {
		if err := initer.Init(host, plugin.Opts); err != nil {
			return fmt.Errorf("kernel: init %s: %w", owner, err)
		}
	}
	return activateCapabilities(plugin.Self, owner, loaded)
}

func activateCapabilities(value any, owner string, loaded *Loaded) error {
	if provider, ok := value.(capabilities.CommandProvider); ok {
		for _, command := range provider.Commands() {
			if err := loaded.Registry.Register(capabilities.RegistryCommand, command.Name, command, owner); err != nil {
				return err
			}
		}
	}
	if provider, ok := value.(capabilities.ComponentProvider); ok {
		for _, component := range provider.Components() {
			if component == nil {
				continue
			}
			if err := loaded.Registry.Register(capabilities.RegistryComponent, component.Name(), component, owner); err != nil {
				return err
			}
		}
	}
	if provider, ok := value.(capabilities.LayoutProvider); ok {
		for _, layout := range provider.Layouts() {
			if layout == nil {
				continue
			}
			if err := loaded.Registry.Register(capabilities.RegistryLayout, layout.Name(), layout, owner); err != nil {
				return err
			}
		}
	}
	if provider, ok := value.(capabilities.BackendProvider); ok {
		for _, backend := range provider.Backends() {
			if err := loaded.Registry.Register(capabilities.RegistryBackend, backend.Name, backend, owner); err != nil {
				return err
			}
		}
	}
	if hook, ok := value.(capabilities.OnMessage); ok {
		loaded.Hooks.OnMessage = append(loaded.Hooks.OnMessage, hook)
	}
	if hook, ok := value.(capabilities.OnSessionStart); ok {
		loaded.Hooks.OnSessionStart = append(loaded.Hooks.OnSessionStart, hook)
	}
	if hook, ok := value.(capabilities.OnToolCall); ok {
		loaded.Hooks.OnToolCall = append(loaded.Hooks.OnToolCall, hook)
	}
	return nil
}
