package plugintest

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/config"
)

// Check validates the common plugin contract. It is intentionally a smoke test:
// it exercises Init and verifies contributed registry entries have stable,
// non-empty names without starting the full plums runtime.
func Check(t testing.TB, plugin config.Plugin) {
	t.Helper()
	for _, err := range CheckPlugin(plugin) {
		t.Errorf("%v", err)
	}
}

// Run is the command-line form of CheckPlugin. It returns 0 when the plugin
// passes and 1 when any contract check fails.
func Run(plugin config.Plugin) int {
	errs := CheckPlugin(plugin)
	if len(errs) == 0 {
		fmt.Fprintln(os.Stdout, "plugin check passed")
		return 0
	}
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "plugin check failed: %v\n", err)
	}
	return 1
}

func CheckPlugin(plugin config.Plugin) []error {
	var errs []error
	if plugin.Self == nil {
		return []error{fmt.Errorf("plugin Self is nil")}
	}
	pl, ok := plugin.Self.(capabilities.Plugin)
	if !ok {
		return []error{fmt.Errorf("plugin %T does not implement capabilities.Plugin", plugin.Self)}
	}
	if strings.TrimSpace(pl.Name()) == "" {
		errs = append(errs, fmt.Errorf("plugin name is empty"))
	}
	if err := pl.Init(fakeHost{}, plugin.Opts); err != nil {
		errs = append(errs, fmt.Errorf("Init failed: %w", err))
	}
	errs = append(errs, checkCommands(plugin.Self)...)
	errs = append(errs, checkComponents(plugin.Self)...)
	errs = append(errs, checkLayouts(plugin.Self)...)
	errs = append(errs, checkBackends(plugin.Self)...)
	return errs
}

func checkCommands(plugin any) []error {
	provider, ok := plugin.(capabilities.CommandProvider)
	if !ok {
		return nil
	}
	var errs []error
	seen := map[string]struct{}{}
	for i, command := range provider.Commands() {
		name := strings.TrimSpace(command.Name)
		if name == "" {
			errs = append(errs, fmt.Errorf("command[%d] name is empty", i))
			continue
		}
		if _, ok := seen[name]; ok {
			errs = append(errs, fmt.Errorf("command %q is registered more than once", name))
		}
		seen[name] = struct{}{}
		if command.Do == nil {
			errs = append(errs, fmt.Errorf("command %q has nil Do", name))
		}
	}
	return errs
}

func checkComponents(plugin any) []error {
	provider, ok := plugin.(capabilities.ComponentProvider)
	if !ok {
		return nil
	}
	var errs []error
	seen := map[string]struct{}{}
	for i, component := range provider.Components() {
		if component == nil {
			errs = append(errs, fmt.Errorf("component[%d] is nil", i))
			continue
		}
		name := strings.TrimSpace(component.Name())
		if name == "" {
			errs = append(errs, fmt.Errorf("component[%d] name is empty", i))
			continue
		}
		if _, ok := seen[name]; ok {
			errs = append(errs, fmt.Errorf("component %q is registered more than once", name))
		}
		seen[name] = struct{}{}
	}
	return errs
}

func checkLayouts(plugin any) []error {
	provider, ok := plugin.(capabilities.LayoutProvider)
	if !ok {
		return nil
	}
	var errs []error
	seen := map[string]struct{}{}
	for i, layout := range provider.Layouts() {
		if layout == nil {
			errs = append(errs, fmt.Errorf("layout[%d] is nil", i))
			continue
		}
		name := strings.TrimSpace(layout.Name())
		if name == "" {
			errs = append(errs, fmt.Errorf("layout[%d] name is empty", i))
			continue
		}
		if _, ok := seen[name]; ok {
			errs = append(errs, fmt.Errorf("layout %q is registered more than once", name))
		}
		seen[name] = struct{}{}
	}
	return errs
}

func checkBackends(plugin any) []error {
	provider, ok := plugin.(capabilities.BackendProvider)
	if !ok {
		return nil
	}
	var errs []error
	seen := map[string]struct{}{}
	for i, backend := range provider.Backends() {
		name := strings.TrimSpace(backend.Name)
		if name == "" {
			errs = append(errs, fmt.Errorf("backend[%d] name is empty", i))
			continue
		}
		if _, ok := seen[name]; ok {
			errs = append(errs, fmt.Errorf("backend %q is registered more than once", name))
		}
		seen[name] = struct{}{}
		if backend.Backend == nil {
			errs = append(errs, fmt.Errorf("backend %q has nil Backend", name))
		}
	}
	return errs
}

type fakeHost struct{}

func (fakeHost) Settings() capabilities.Settings       { return capabilities.Settings{} }
func (fakeHost) Log(string, ...any)                    {}
func (fakeHost) Services() capabilities.Services       { return fakeServices{} }
func (fakeServices) Palette() capabilities.Palette     { return fakePalette{} }
func (fakeServices) Clipboard() capabilities.Clipboard { return fakeClipboard{} }
func (fakeServices) Completion() capabilities.Completion {
	return &fakeCompletion{}
}
func (fakeServices) Selection() capabilities.Selection { return fakeSelection{} }

type fakeServices struct{}

type fakePalette struct{}

func (fakePalette) Open(string, []capabilities.ListItem, func(capabilities.ListItem)) {}
func (fakePalette) OpenCommandPalette()                                               {}

type fakeClipboard struct{}

func (fakeClipboard) Copy(string) error      { return nil }
func (fakeClipboard) Paste() (string, error) { return "", nil }

type fakeCompletion struct {
	sources []capabilities.CompletionSource
}

func (c *fakeCompletion) Register(source capabilities.CompletionSource) {
	c.sources = append([]capabilities.CompletionSource{source}, c.sources...)
}
func (c *fakeCompletion) Sources() []capabilities.CompletionSource { return c.sources }

type fakeSelection struct{}

func (fakeSelection) Current() string { return "" }
func (fakeSelection) Copy() error     { return nil }
