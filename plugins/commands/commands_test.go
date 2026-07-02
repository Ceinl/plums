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

func TestCommandsIncludeSlashAndPaletteCommands(t *testing.T) {
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

func (c *recordCtx) View() capabilities.View          { return recordView{c} }
func (c *recordCtx) Services() capabilities.Services  { return recordServices{c} }
func (c *recordCtx) State() capabilities.CommandState { return c.state }
func (c *recordCtx) Input() capabilities.Editor       { return &recordEditor{text: c.inputText} }
func (c *recordCtx) Send(text string)                 { c.sent = text }

type recordView struct{ c *recordCtx }

func (v recordView) SwitchMode() { v.c.calls = append(v.c.calls, "SwitchMode") }
func (v recordView) CycleThinkingVisibility() {
	v.c.calls = append(v.c.calls, "CycleThinkingVisibility")
}
func (v recordView) CycleToolCallVisibility() {
	v.c.calls = append(v.c.calls, "CycleToolCallVisibility")
}
func (v recordView) SwitchLayout() { v.c.calls = append(v.c.calls, "SwitchLayout") }
func (v recordView) OpenSkills()   { v.c.calls = append(v.c.calls, "OpenSkills") }

type recordServices struct{ c *recordCtx }

func (s recordServices) Palette() capabilities.Palette       { return recordPalette{s.c} }
func (s recordServices) Clipboard() capabilities.Clipboard   { return nil }
func (s recordServices) Completion() capabilities.Completion { return nil }
func (s recordServices) Selection() capabilities.Selection   { return nil }

type recordPalette struct{ c *recordCtx }

func (p recordPalette) Open(string, []capabilities.ListItem, func(capabilities.ListItem)) {}
func (p recordPalette) OpenCommandPalette() {
	p.c.calls = append(p.c.calls, "OpenCommandPalette")
}

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
