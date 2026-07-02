package app

import (
	"context"

	"github.com/Ceinl/plums/capabilities"
	backendcommands "github.com/Ceinl/plums/plugins/backend"
	"github.com/Ceinl/plums/plugins/commands"
	"github.com/Ceinl/plums/plugins/sessions"
	"github.com/Ceinl/plums/plugins/skills"
)

// builtinCommands returns the bundled command set exactly as the Default Config
// wires it, so app tests exercise the real command plugin rather than a stub.
func builtinCommands() []capabilities.Command {
	out := (&backendcommands.Plugin{}).Commands()
	out = append(out, (&sessions.Plugin{}).Commands()...)
	out = append(out, (&commands.Plugin{}).Commands()...)
	out = append(out, (&skills.Plugin{}).Commands()...)
	return out
}

// stateWithBuiltinCommands builds a State seeded with the bundled commands, the
// post-Phase-4 equivalent of the old default command config.
func stateWithBuiltinCommands(w, h int) *State {
	state := NewState(w, h)
	state.SetCommands(builtinCommands())
	return state
}

// fakeCtx records which command verbs a command's Do invoked, so a test can run
// a pending command and assert the effect without the live run loop.
type fakeCtx struct {
	calls  []string
	copied string
}

func (c *fakeCtx) record(name string) { c.calls = append(c.calls, name) }
func (c *fakeCtx) called(name string) bool {
	for _, call := range c.calls {
		if call == name {
			return true
		}
	}
	return false
}

func (c *fakeCtx) Session() capabilities.Session { return capabilities.Session{} }
func (c *fakeCtx) Input() capabilities.Editor    { return nil }
func (c *fakeCtx) Selection() string             { return "" }
func (c *fakeCtx) Send(string)                   {}
func (c *fakeCtx) Chat(string, string)           {}
func (c *fakeCtx) Copy(text string)              { c.copied = text }
func (c *fakeCtx) Shell(context.Context, string, ...string) (string, error) {
	return "", nil
}
func (c *fakeCtx) SetLayout(string)                                                      {}
func (c *fakeCtx) OpenList(string, []capabilities.ListItem, func(capabilities.ListItem)) {}
func (c *fakeCtx) Services() capabilities.Services                                       { return fakeCtxServices{c} }

func (c *fakeCtx) View() capabilities.View         { return fakeCtxView{c} }
func (c *fakeCtx) Backends() capabilities.Backends { return fakeCtxBackends{c} }
func (c *fakeCtx) Sessions() capabilities.Sessions { return fakeCtxSessions{c} }
func (c *fakeCtx) State() capabilities.CommandState {
	return capabilities.CommandState{}
}

type fakeCtxView struct{ c *fakeCtx }

func (v fakeCtxView) SwitchMode()              { v.c.record("SwitchMode") }
func (v fakeCtxView) CycleThinkingVisibility() { v.c.record("CycleThinkingVisibility") }
func (v fakeCtxView) CycleToolCallVisibility() { v.c.record("CycleToolCallVisibility") }
func (v fakeCtxView) SwitchLayout()            { v.c.record("SwitchLayout") }
func (v fakeCtxView) OpenSkills()              { v.c.record("OpenSkills") }

type fakeCtxBackends struct{ c *fakeCtx }

func (b fakeCtxBackends) Switch()                 { b.c.record("SwitchBackend") }
func (b fakeCtxBackends) Select(string)           { b.c.record("SelectBackend") }
func (b fakeCtxBackends) ChangeModel()            { b.c.record("ChangeModel") }
func (b fakeCtxBackends) SetModel(string, string) { b.c.record("SetModel") }
func (b fakeCtxBackends) AnswerQuestion(string)   { b.c.record("AnswerQuestion") }

type fakeCtxSessions struct{ c *fakeCtx }

func (s fakeCtxSessions) New()        { s.c.record("NewSession") }
func (s fakeCtxSessions) Picker()     { s.c.record("OpenSessions") }
func (s fakeCtxSessions) Open(string) { s.c.record("OpenSession") }

type fakeCtxServices struct{ c *fakeCtx }

func (s fakeCtxServices) Palette() capabilities.Palette       { return fakeCtxPalette{s.c} }
func (s fakeCtxServices) Clipboard() capabilities.Clipboard   { return nil }
func (s fakeCtxServices) Completion() capabilities.Completion { return nil }
func (s fakeCtxServices) Selection() capabilities.Selection   { return nil }

type fakeCtxPalette struct{ c *fakeCtx }

func (p fakeCtxPalette) Open(string, []capabilities.ListItem, func(capabilities.ListItem)) {}
func (p fakeCtxPalette) OpenCommandPalette()                                               { p.c.record("OpenCommandPalette") }

// runPending consumes the state's pending command and runs its Do against a
// fakeCtx, returning the recorder.
func runPending(state *State) *fakeCtx {
	ctx := &fakeCtx{}
	command, ok := state.ConsumePendingCommand()
	if !ok || command.Do == nil {
		return ctx
	}
	_ = command.Do(context.Background(), ctx)
	return ctx
}
