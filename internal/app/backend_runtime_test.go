package app

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

type backendRuntimeTestBackend struct{}

func (backendRuntimeTestBackend) Health(context.Context) error { return nil }

func (backendRuntimeTestBackend) CreateSession(context.Context, string) (*capabilities.Session, error) {
	return nil, nil
}

func (backendRuntimeTestBackend) ListSessions(context.Context) ([]capabilities.Session, error) {
	return nil, nil
}

func (backendRuntimeTestBackend) GetSession(context.Context, string) (*capabilities.Session, error) {
	return nil, nil
}

func (backendRuntimeTestBackend) ListMessages(context.Context, string) ([]capabilities.MessageResponse, error) {
	return nil, nil
}

func (backendRuntimeTestBackend) ListProviders(context.Context) ([]capabilities.Provider, []string, error) {
	return nil, nil, nil
}

func (backendRuntimeTestBackend) SendMessageEvents(context.Context, string, string, string, string, string) <-chan capabilities.StreamEvent {
	ch := make(chan capabilities.StreamEvent)
	close(ch)
	return ch
}

func (backendRuntimeTestBackend) ReplyQuestion(context.Context, string, [][]string) error {
	return nil
}

func TestBackendRuntimesFromRegistrations(t *testing.T) {
	backend := backendRuntimeTestBackend{}
	startup := func(context.Context, capabilities.Backend) (*capabilities.StartupResult, error) {
		return &capabilities.StartupResult{}, nil
	}

	runtimes := BackendRuntimesFromRegistrations([]capabilities.BackendRegistration{
		{Name: "opencode", Label: "OpenCode", Backend: backend, Startup: startup},
		{Name: "codex", Backend: backend},
		{Name: "", Backend: backend},
		{Name: "empty"},
	})

	if len(runtimes) != 2 {
		t.Fatalf("runtime count = %d, want 2", len(runtimes))
	}
	if runtimes[0].ID != "opencode" || runtimes[0].Name != "OpenCode" || runtimes[0].Backend == nil || runtimes[0].Startup == nil {
		t.Fatalf("first runtime = %+v", runtimes[0])
	}
	if runtimes[1].ID != "codex" || runtimes[1].Name != "codex" {
		t.Fatalf("second runtime = %+v", runtimes[1])
	}
}
