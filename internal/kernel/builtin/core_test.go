package builtin

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

type testBackend struct{}

func (testBackend) Health(context.Context) error { return nil }

func (testBackend) CreateSession(context.Context, string) (*capabilities.Session, error) {
	return nil, nil
}

func (testBackend) ListSessions(context.Context) ([]capabilities.Session, error) {
	return nil, nil
}

func (testBackend) GetSession(context.Context, string) (*capabilities.Session, error) {
	return nil, nil
}

func (testBackend) ListMessages(context.Context, string) ([]capabilities.MessageResponse, error) {
	return nil, nil
}

func (testBackend) ListProviders(context.Context) ([]capabilities.Provider, []string, error) {
	return nil, nil, nil
}

func (testBackend) SendMessageEvents(context.Context, string, string, string, string, string) <-chan capabilities.StreamEvent {
	ch := make(chan capabilities.StreamEvent)
	close(ch)
	return ch
}

func (testBackend) ReplyQuestion(context.Context, string, [][]string) error {
	return nil
}

type testComponent struct {
	name string
}

func (c testComponent) Name() string                                        { return c.name }
func (c testComponent) Arrange(capabilities.Rect)                           {}
func (c testComponent) Render(capabilities.RenderCtx, capabilities.Surface) {}

func TestDefaultPluginsHasPerFeatureOwners(t *testing.T) {
	plugins := DefaultPlugins(CoreOptions{
		WorkingDirectory:  "/tmp/project",
		OpencodeServerURL: "http://127.0.0.1:4096",
	})
	names := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		named, ok := plugin.Self.(interface{ Name() string })
		if !ok || named.Name() == "" {
			t.Fatalf("plugin has empty name: %T", plugin.Self)
		}
		names = append(names, named.Name())
	}
	// Layouts and commands no longer ship from Core — they are public plugins
	// wired by the Default Config (internal/builtincfg), so "ui/layouts" and
	// "ui/commands" are absent here.
	want := []string{
		"backend/opencode", "backend/codex", "backend/claude", "backend/claude-mirror",
		"ui/components",
	}
	if len(names) != len(want) {
		t.Fatalf("plugin names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("plugin names = %v, want %v", names, want)
		}
	}
}

func TestBackendPluginExposesSingleRegistration(t *testing.T) {
	plugin := backendPlugin{
		name:         "backend/opencode",
		registration: capabilities.BackendRegistration{Name: "opencode", Backend: testBackend{}},
	}
	backends := plugin.Backends()
	if len(backends) != 1 || backends[0].Name != "opencode" {
		t.Fatalf("Backends() = %+v", backends)
	}
}

func TestDefaultComponents(t *testing.T) {
	components := DefaultComponents()
	names := make([]string, 0, len(components))
	for _, component := range components {
		names = append(names, component.Name())
	}
	for _, want := range []string{"chat_output", "editor", "status_bar"} {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("component names = %v, missing %s", names, want)
		}
	}
}

func TestComponentsPluginCopiesDefaults(t *testing.T) {
	plugin := componentsPlugin{components: DefaultComponents()}
	components := plugin.Components()
	if len(components) == 0 {
		t.Fatal("expected default components")
	}

	components[0] = testComponent{name: "changed"}
	again := plugin.Components()
	if again[0].Name() != "chat_output" {
		t.Fatalf("Components() exposed mutable slice: %+v", again)
	}
}
