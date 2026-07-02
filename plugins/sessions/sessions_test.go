package sessions

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

type recordCtx struct {
	capabilities.Ctx
	calls []string
}

func (c *recordCtx) Sessions() capabilities.Sessions { return recordSessions{c} }

type recordSessions struct{ c *recordCtx }

func (s recordSessions) New()        { s.c.calls = append(s.c.calls, "NewSession") }
func (s recordSessions) Picker()     { s.c.calls = append(s.c.calls, "OpenSessions") }
func (s recordSessions) Open(string) { s.c.calls = append(s.c.calls, "OpenSession") }

func TestCommandsOwnSessionEntries(t *testing.T) {
	commands := (&Plugin{}).Commands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		if command.Do == nil {
			t.Fatalf("command %q has nil Do", command.Name)
		}
		names = append(names, command.Name)
	}
	want := []string{"/new", "/sessions", "Start new session", "Sessions list"}
	if len(names) != len(want) {
		t.Fatalf("commands = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("commands = %v, want %v", names, want)
		}
	}
}

func TestSlashNewCallsNewSession(t *testing.T) {
	var command capabilities.Command
	for _, candidate := range (&Plugin{}).Commands() {
		if candidate.Name == "/new" {
			command = candidate
			break
		}
	}
	ctx := &recordCtx{}
	if err := command.Do(context.Background(), ctx); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(ctx.calls) != 1 || ctx.calls[0] != "NewSession" {
		t.Fatalf("calls = %v, want [NewSession]", ctx.calls)
	}
}

func TestSessionsListCallsOpenSessions(t *testing.T) {
	var command capabilities.Command
	for _, candidate := range (&Plugin{}).Commands() {
		if candidate.Name == "Sessions list" {
			command = candidate
			break
		}
	}
	ctx := &recordCtx{}
	if err := command.Do(context.Background(), ctx); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(ctx.calls) != 1 || ctx.calls[0] != "OpenSessions" {
		t.Fatalf("calls = %v, want [OpenSessions]", ctx.calls)
	}
}
