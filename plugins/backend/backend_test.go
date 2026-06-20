package backend

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

type recordCtx struct {
	capabilities.Ctx
	calls []string
}

func (c *recordCtx) SwitchBackend() { c.calls = append(c.calls, "SwitchBackend") }
func (c *recordCtx) ChangeModel()   { c.calls = append(c.calls, "ChangeModel") }

func TestCommandsOwnBackendAndModelEntries(t *testing.T) {
	commands := (&Plugin{}).Commands()
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		if command.Do == nil {
			t.Fatalf("command %q has nil Do", command.Name)
		}
		names = append(names, command.Name)
	}
	want := []string{"/backend", "/model", "Change model", "Backend provider"}
	if len(names) != len(want) {
		t.Fatalf("commands = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("commands = %v, want %v", names, want)
		}
	}
}

func TestSlashModelCallsChangeModel(t *testing.T) {
	var command capabilities.Command
	for _, candidate := range (&Plugin{}).Commands() {
		if candidate.Name == "/model" {
			command = candidate
			break
		}
	}
	ctx := &recordCtx{}
	if err := command.Do(context.Background(), ctx); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(ctx.calls) != 1 || ctx.calls[0] != "ChangeModel" {
		t.Fatalf("calls = %v, want [ChangeModel]", ctx.calls)
	}
}

func TestBackendTitleReflectsCurrentBackend(t *testing.T) {
	var command capabilities.Command
	for _, candidate := range (&Plugin{}).Commands() {
		if candidate.Name == "Backend provider" {
			command = candidate
			break
		}
	}
	label := command.Title(capabilities.CommandState{BackendProvider: "opencode"})
	if label.Title != "Backend provider" || label.Detail != "Current backend: opencode" {
		t.Fatalf("label = %+v", label)
	}
}
