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
func (c *fakeCtx) Services() capabilities.Services                                       { return nil }

func (c *fakeCtx) OpenCommandPalette()      { c.record("OpenCommandPalette") }
func (c *fakeCtx) ChangeModel()             { c.record("ChangeModel") }
func (c *fakeCtx) SwitchBackend()           { c.record("SwitchBackend") }
func (c *fakeCtx) NewSession()              { c.record("NewSession") }
func (c *fakeCtx) OpenSessions()            { c.record("OpenSessions") }
func (c *fakeCtx) OpenSkills()              { c.record("OpenSkills") }
func (c *fakeCtx) SwitchMode()              { c.record("SwitchMode") }
func (c *fakeCtx) CycleThinkingVisibility() { c.record("CycleThinkingVisibility") }
func (c *fakeCtx) CycleToolCallVisibility() { c.record("CycleToolCallVisibility") }
func (c *fakeCtx) AdjustOutputPercent(int)  { c.record("AdjustOutputPercent") }
func (c *fakeCtx) SwitchLayout()            { c.record("SwitchLayout") }
func (c *fakeCtx) State() capabilities.CommandState {
	return capabilities.CommandState{}
}

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
