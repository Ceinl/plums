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

type testLayout struct {
	name string
}

func (l testLayout) Name() string            { return l.name }
func (l testLayout) Tree() capabilities.Node { return nil }

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
	want := []string{
		"backend/opencode", "backend/codex", "backend/claude", "backend/claude-mirror",
		"ui/components", "ui/layouts", "ui/commands",
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

func TestBackendRegistrations(t *testing.T) {
	registrations := BackendRegistrations(CoreOptions{
		WorkingDirectory:  "/tmp/project",
		OpencodeServerURL: "http://127.0.0.1:4096",
	})

	names := make([]string, 0, len(registrations))
	for _, registration := range registrations {
		if registration.Backend == nil {
			t.Fatalf("%s backend is nil", registration.Name)
		}
		if registration.Startup == nil {
			t.Fatalf("%s startup is nil", registration.Name)
		}
		names = append(names, registration.Name)
	}
	want := []string{"opencode", "codex", "claude", "claude-mirror"}
	if len(names) != len(want) {
		t.Fatalf("backend names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("backend names = %v, want %v", names, want)
		}
	}
}

func TestDefaultCommands(t *testing.T) {
	commands := DefaultCommands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		if command.Detail == "" {
			t.Fatalf("%s command has empty detail", command.Name)
		}
		names = append(names, command.Name)
	}
	want := []string{"/new", "/command", "/backend", "/skills", "/sessions", "/model"}
	if len(names) != len(want) {
		t.Fatalf("command names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("command names = %v, want %v", names, want)
		}
	}
}

func TestCommandsPluginCopiesDefaults(t *testing.T) {
	plugin := commandsPlugin{commands: DefaultCommands()}
	commands := plugin.Commands()
	if len(commands) == 0 {
		t.Fatal("expected default commands")
	}

	commands[0].Name = "/changed"
	again := plugin.Commands()
	if again[0].Name != "/new" {
		t.Fatalf("Commands() exposed mutable slice: %+v", again)
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

func TestDefaultLayouts(t *testing.T) {
	layouts := DefaultLayouts()
	names := make([]string, 0, len(layouts))
	for _, layout := range layouts {
		names = append(names, layout.Name())
	}
	// split / narrow_split intentionally ship as a user plugin, not a builtin.
	want := []string{"chat", "zen", "narrow_chat", "fullscreen"}
	if len(names) != len(want) {
		t.Fatalf("layout names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("layout names = %v, want %v", names, want)
		}
	}
}

func TestLayoutsPluginCopiesDefaults(t *testing.T) {
	plugin := layoutsPlugin{layouts: DefaultLayouts()}
	layouts := plugin.Layouts()
	if len(layouts) == 0 {
		t.Fatal("expected default layouts")
	}

	layouts[0] = testLayout{name: "changed"}
	again := plugin.Layouts()
	if again[0].Name() != "chat" {
		t.Fatalf("Layouts() exposed mutable slice: %+v", again)
	}
}
