package opencode

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestDefaultPortForDirUsesEphemeralRange(t *testing.T) {
	port := DefaultPortForDir("/tmp/project")
	if port < 49152 || port > 65535 {
		t.Fatalf("port = %d, want ephemeral range 49152-65535", port)
	}
}

func TestDefaultBaseURLForDirUsesDerivedPort(t *testing.T) {
	got := DefaultBaseURLForDir("/tmp/project")
	want := "http://127.0.0.1:"
	if len(got) <= len(want) || got[:len(want)] != want {
		t.Fatalf("base URL = %q, want prefix %q", got, want)
	}
}

type healthyBackend struct{}

func (healthyBackend) Health(context.Context) error { return nil }

func (healthyBackend) CreateSession(context.Context, string) (*capabilities.Session, error) {
	return nil, nil
}

func (healthyBackend) ListSessions(context.Context) ([]capabilities.Session, error) {
	return nil, nil
}

func (healthyBackend) GetSession(context.Context, string) (*capabilities.Session, error) {
	return nil, nil
}

func (healthyBackend) ListMessages(context.Context, string) ([]capabilities.MessageResponse, error) {
	return nil, nil
}

func (healthyBackend) ListProviders(context.Context) ([]capabilities.Provider, []string, error) {
	return nil, nil, nil
}

func (healthyBackend) SendMessageEvents(context.Context, string, string, string, string, string) <-chan capabilities.StreamEvent {
	ch := make(chan capabilities.StreamEvent)
	close(ch)
	return ch
}

func (healthyBackend) ReplyQuestion(context.Context, string, [][]string) error {
	return nil
}

func TestStartupExistingHealthyServerReturnsNilServerProcess(t *testing.T) {
	start := startup("/tmp/project", 0, nil)

	result, err := start(context.Background(), healthyBackend{})
	if err != nil {
		t.Fatalf("startup() error = %v", err)
	}
	if result == nil {
		t.Fatal("startup() returned nil result")
	}
	if result.Server != nil {
		t.Fatalf("server process = %#v, want nil interface for existing server", result.Server)
	}
}
