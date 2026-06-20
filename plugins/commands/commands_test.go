package commands

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestPluginName(t *testing.T) {
	if name := (&Plugin{}).Name(); name != "ui/commands" {
		t.Fatalf("Name() = %q, want ui/commands", name)
	}
}

func TestCommandsIncludeSlashAndPaletteActions(t *testing.T) {
	got := (&Plugin{}).Commands()
	names := make([]string, 0, len(got))
	for _, c := range got {
		if c.Do == nil {
			t.Fatalf("command %q has no Do", c.Name)
		}
		names = append(names, c.Name)
	}
	want := []string{
		"/new", "/command", "/backend", "/skills", "/sessions", "/model",
		"Change model", "Backend provider", "Start new session", "Switch mode",
		"Layouts", "Thinking visibility", "Tool call visibility", "Output percentage",
		"Skills list", "Sessions list",
	}
	if len(names) != len(want) {
		t.Fatalf("command names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("command names = %v, want %v", names, want)
		}
	}
}

// recordCtx records command verb invocations.
type recordCtx struct {
	capabilities.Ctx
	calls []string
	state capabilities.CommandState
}

func (c *recordCtx) NewSession()                      { c.calls = append(c.calls, "NewSession") }
func (c *recordCtx) OpenCommandPalette()              { c.calls = append(c.calls, "OpenCommandPalette") }
func (c *recordCtx) SwitchBackend()                   { c.calls = append(c.calls, "SwitchBackend") }
func (c *recordCtx) OpenSkills()                      { c.calls = append(c.calls, "OpenSkills") }
func (c *recordCtx) OpenSessions()                    { c.calls = append(c.calls, "OpenSessions") }
func (c *recordCtx) ChangeModel()                     { c.calls = append(c.calls, "ChangeModel") }
func (c *recordCtx) SwitchMode()                      { c.calls = append(c.calls, "SwitchMode") }
func (c *recordCtx) SwitchLayout()                    { c.calls = append(c.calls, "SwitchLayout") }
func (c *recordCtx) CycleThinkingVisibility()         { c.calls = append(c.calls, "CycleThinkingVisibility") }
func (c *recordCtx) CycleToolCallVisibility()         { c.calls = append(c.calls, "CycleToolCallVisibility") }
func (c *recordCtx) State() capabilities.CommandState { return c.state }

func TestSlashNewCallsNewSession(t *testing.T) {
	var cmd capabilities.Command
	for _, c := range (&Plugin{}).Commands() {
		if c.Name == "/new" {
			cmd = c
		}
	}
	ctx := &recordCtx{}
	if err := cmd.Do(context.Background(), ctx); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(ctx.calls) != 1 || ctx.calls[0] != "NewSession" {
		t.Fatalf("calls = %v, want [NewSession]", ctx.calls)
	}
}

func TestDynamicTitlesReflectState(t *testing.T) {
	var switchMode, backend, layouts capabilities.Command
	for _, c := range (&Plugin{}).Commands() {
		switch c.Name {
		case "Switch mode":
			switchMode = c
		case "Backend provider":
			backend = c
		case "Layouts":
			layouts = c
		}
	}
	plan := switchMode.Title(capabilities.CommandState{Mode: "plan"})
	if plan.Title != "Switch to build mode" || plan.Detail != "Current mode: plan" {
		t.Fatalf("plan title = %+v", plan)
	}
	build := switchMode.Title(capabilities.CommandState{Mode: "build"})
	if build.Title != "Switch to plan mode" {
		t.Fatalf("build title = %+v", build)
	}
	b := backend.Title(capabilities.CommandState{BackendProvider: "opencode"})
	if b.Detail != "Current backend: opencode" {
		t.Fatalf("backend detail = %q", b.Detail)
	}
	l := layouts.Title(capabilities.CommandState{Layout: "split"})
	if l.Detail != "Select layout - current: split" {
		t.Fatalf("layouts detail = %q", l.Detail)
	}
}

func TestSlashCommandsAreHiddenFromPalette(t *testing.T) {
	for _, c := range (&Plugin{}).Commands() {
		if c.Name == "/new" || c.Name == "/model" {
			if label := c.Title(capabilities.CommandState{}); !label.Disabled {
				t.Fatalf("slash command %q should be palette-disabled", c.Name)
			}
		}
	}
}
