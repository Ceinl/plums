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
		"palette.open", "prompt.submit",
		"/command",
		"Switch mode",
		"Layouts", "Thinking visibility", "Tool call visibility", "Output percentage",
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
	calls     []string
	state     capabilities.CommandState
	inputText string
	sent      string
}

func (c *recordCtx) OpenCommandPalette()              { c.calls = append(c.calls, "OpenCommandPalette") }
func (c *recordCtx) SwitchMode()                      { c.calls = append(c.calls, "SwitchMode") }
func (c *recordCtx) SwitchLayout()                    { c.calls = append(c.calls, "SwitchLayout") }
func (c *recordCtx) CycleThinkingVisibility()         { c.calls = append(c.calls, "CycleThinkingVisibility") }
func (c *recordCtx) CycleToolCallVisibility()         { c.calls = append(c.calls, "CycleToolCallVisibility") }
func (c *recordCtx) State() capabilities.CommandState { return c.state }
func (c *recordCtx) Input() capabilities.Editor       { return &recordEditor{text: c.inputText} }
func (c *recordCtx) Send(text string)                 { c.sent = text }

type recordEditor struct {
	text string
}

func (e *recordEditor) Text() string     { return e.text }
func (e *recordEditor) SetText(v string) { e.text = v }

func TestPaletteOpenCommandCallsOpenCommandPalette(t *testing.T) {
	var cmd capabilities.Command
	for _, c := range (&Plugin{}).Commands() {
		if c.Name == "palette.open" {
			cmd = c
		}
	}
	ctx := &recordCtx{}
	if err := cmd.Do(context.Background(), ctx); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(ctx.calls) != 1 || ctx.calls[0] != "OpenCommandPalette" {
		t.Fatalf("calls = %v, want [OpenCommandPalette]", ctx.calls)
	}
}

func TestPromptSubmitCommandSendsInputText(t *testing.T) {
	var cmd capabilities.Command
	for _, c := range (&Plugin{}).Commands() {
		if c.Name == "prompt.submit" {
			cmd = c
		}
	}
	ctx := &recordCtx{inputText: "ship it"}
	if err := cmd.Do(context.Background(), ctx); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if ctx.sent != "ship it" {
		t.Fatalf("sent = %q, want ship it", ctx.sent)
	}
}

func TestDynamicTitlesReflectState(t *testing.T) {
	var switchMode, layouts capabilities.Command
	for _, c := range (&Plugin{}).Commands() {
		switch c.Name {
		case "Switch mode":
			switchMode = c
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
	l := layouts.Title(capabilities.CommandState{Layout: "split"})
	if l.Detail != "Select layout - current: split" {
		t.Fatalf("layouts detail = %q", l.Detail)
	}
}

func TestNonPaletteCommandsAreHiddenFromPalette(t *testing.T) {
	for _, c := range (&Plugin{}).Commands() {
		if c.Name == "palette.open" || c.Name == "prompt.submit" || c.Name == "/command" {
			if label := c.Title(capabilities.CommandState{}); !label.Disabled {
				t.Fatalf("command %q should be palette-disabled", c.Name)
			}
		}
	}
}
