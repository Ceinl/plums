package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Ceinl/plums/capabilities"
)

type sessionTestBackend struct {
	sessions      []capabilities.Session
	listErr       error
	created       *capabilities.Session
	createCalls   int
	createDir     string
	listProviders int
}

func (b *sessionTestBackend) Health(ctx context.Context) error { return nil }

func (b *sessionTestBackend) CreateSession(ctx context.Context, directory string) (*capabilities.Session, error) {
	b.createCalls++
	b.createDir = directory
	if b.created != nil {
		return b.created, nil
	}
	return &capabilities.Session{ID: "created", Directory: directory}, nil
}

func (b *sessionTestBackend) ListSessions(ctx context.Context) ([]capabilities.Session, error) {
	if b.listErr != nil {
		return nil, b.listErr
	}
	return b.sessions, nil
}

func (b *sessionTestBackend) GetSession(ctx context.Context, sessionID string) (*capabilities.Session, error) {
	return nil, errors.New("not implemented")
}

func (b *sessionTestBackend) ListMessages(ctx context.Context, sessionID string) ([]capabilities.MessageResponse, error) {
	return nil, errors.New("not implemented")
}

func (b *sessionTestBackend) ListProviders(ctx context.Context) ([]capabilities.Provider, []string, error) {
	b.listProviders++
	return nil, nil, errors.New("not implemented")
}

func (b *sessionTestBackend) SendMessageEvents(ctx context.Context, sessionID, text, providerID, modelID, agent string) <-chan capabilities.StreamEvent {
	ch := make(chan capabilities.StreamEvent)
	close(ch)
	return ch
}

func (b *sessionTestBackend) ReplyQuestion(ctx context.Context, requestID string, answers [][]string) error {
	return nil
}

func TestEnsureSessionReusesLatestSessionForWorkingDirectory(t *testing.T) {
	backend := &sessionTestBackend{sessions: []capabilities.Session{
		{ID: "other", Directory: "/tmp/other", Time: capabilities.SessionTime{Updated: 300}},
		{ID: "old", Title: "Old", Directory: "/tmp/project", Time: capabilities.SessionTime{Updated: 100}},
		{ID: "new", Title: "New", Directory: "/tmp/project", Model: &capabilities.ModelRef{ProviderID: "openai", ID: "gpt"}, Time: capabilities.SessionTime{Updated: 200}},
	}}
	state := NewState(80, 24)
	cfg := RunConfig{WorkingDirectory: "/tmp/project", ListTimeout: time.Second, RecentModelTimeout: time.Second}

	if err := ensureSession(context.Background(), state, backend, cfg); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if backend.createCalls != 0 {
		t.Fatalf("expected existing project session to be reused, created %d sessions", backend.createCalls)
	}
	if state.SessionID != "new" || state.SessionTitle != "New" {
		t.Fatalf("expected newest project session, got id=%q title=%q", state.SessionID, state.SessionTitle)
	}
	if state.ModelProvider != "openai" || state.ModelID != "gpt" {
		t.Fatalf("expected model from reused session, got %q/%q", state.ModelProvider, state.ModelID)
	}
}

func TestEnsureSessionCreatesWhenNoWorkingDirectoryMatch(t *testing.T) {
	backend := &sessionTestBackend{sessions: []capabilities.Session{{ID: "other", Directory: "/tmp/other"}}}
	state := NewState(80, 24)
	cfg := RunConfig{WorkingDirectory: "/tmp/project", ListTimeout: time.Second, RecentModelTimeout: time.Second}

	if err := ensureSession(context.Background(), state, backend, cfg); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if backend.createCalls != 1 || backend.createDir != "/tmp/project" {
		t.Fatalf("expected create for working directory, calls=%d dir=%q", backend.createCalls, backend.createDir)
	}
	if state.SessionID != "created" {
		t.Fatalf("expected created session, got %q", state.SessionID)
	}
}

func TestEnsureSessionClearHistorySkipsExistingSessions(t *testing.T) {
	backend := &sessionTestBackend{sessions: []capabilities.Session{
		{ID: "old", Title: "Old", Directory: "/tmp/project", Time: capabilities.SessionTime{Updated: 100}},
	}}
	state := NewState(80, 24)
	cfg := RunConfig{WorkingDirectory: "/tmp/project", ListTimeout: time.Second, RecentModelTimeout: time.Second, ClearHistory: true}

	if err := ensureSession(context.Background(), state, backend, cfg); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if backend.createCalls != 1 || state.SessionID != "created" {
		t.Fatalf("expected fresh session despite existing history, calls=%d session=%q", backend.createCalls, state.SessionID)
	}
	if !state.IsRunSession("created") {
		t.Fatalf("expected created session to be marked as run session")
	}
}

func TestListSessionItemsClearHistoryFiltersToRunSessions(t *testing.T) {
	backend := &sessionTestBackend{sessions: []capabilities.Session{
		{ID: "old", Title: "Old", Directory: "/tmp/project", Time: capabilities.SessionTime{Updated: 100}},
		{ID: "mine", Title: "Mine", Directory: "/tmp/project", Time: capabilities.SessionTime{Updated: 200}},
	}}
	state := NewState(80, 24)
	state.MarkRunSession("mine")
	cfg := RunConfig{ListTimeout: time.Second, ClearHistory: true}

	items, err := listSessionItems(context.Background(), state, backend, cfg)
	if err != nil {
		t.Fatalf("list session items: %v", err)
	}
	if len(items) != 1 || items[0].ID != "mine" {
		t.Fatalf("expected only run session, got %#v", items)
	}

	cfg.ClearHistory = false
	items, err = listSessionItems(context.Background(), state, backend, cfg)
	if err != nil {
		t.Fatalf("list session items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected full history without ClearHistory, got %#v", items)
	}
}

func TestEnsureSessionCreatesWhenStartupLookupFails(t *testing.T) {
	backend := &sessionTestBackend{listErr: errors.New("boom")}
	state := NewState(80, 24)
	cfg := RunConfig{WorkingDirectory: "/tmp/project", ListTimeout: time.Second, RecentModelTimeout: time.Second}

	if err := ensureSession(context.Background(), state, backend, cfg); err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if backend.createCalls != 1 || state.SessionID != "created" {
		t.Fatalf("expected fallback create, calls=%d session=%q", backend.createCalls, state.SessionID)
	}
}
