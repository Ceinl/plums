package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Ceinl/plums/capabilities"
)

type stateMutation func(*State)

type runtimeCtx struct {
	session      capabilities.Session
	input        *runtimeEditor
	selection    string
	workingDir   string
	shellTimeout time.Duration
	clipboardCmd string
	mutations    chan<- stateMutation
	prompts      chan<- string
	completion   *completionRegistry
	commandState capabilities.CommandState
}

func newRuntimeCtx(state *State, cfg RunConfig, mutations chan<- stateMutation, prompts chan<- string) *runtimeCtx {
	selection := ""
	if state.Editor != nil && state.Editor.HasSelection() {
		selection = state.Editor.SelectedText()
	} else {
		selection = state.PublicComponentSelection()
	}
	shellTimeout := cfg.ListTimeout
	if shellTimeout <= 0 {
		shellTimeout = 30 * time.Second
	}
	return &runtimeCtx{
		session: capabilities.Session{
			ID:    state.SessionID,
			Title: state.SessionTitle,
			Model: modelRefFromState(state),
		},
		input:        &runtimeEditor{text: state.Editor.GetContent(), mutations: mutations},
		selection:    selection,
		workingDir:   cfg.WorkingDirectory,
		shellTimeout: shellTimeout,
		clipboardCmd: cfg.ClipboardCommand,
		mutations:    mutations,
		prompts:      prompts,
		commandState: commandStateFromState(state),
	}
}

// commandStateFromState snapshots the host fields commands read to render
// dynamic palette labels. It mirrors the legacy commands.json template vars.
func commandStateFromState(state *State) capabilities.CommandState {
	return capabilities.CommandState{
		Mode:               state.Mode,
		Layout:             state.LayoutLabel(),
		ThinkingVisibility: state.ThinkingVisibilityLabel(),
		ToolCallVisibility: state.ToolCallVisibilityLabel(),
		OutputPercent:      state.SplitOutputPercent(),
		BackendProvider:    state.BackendProvider,
	}
}

func modelRefFromState(state *State) *capabilities.ModelRef {
	if state.ModelID == "" && state.ModelProvider == "" {
		return nil
	}
	return &capabilities.ModelRef{ID: state.ModelID, ProviderID: state.ModelProvider}
}

func (c *runtimeCtx) Session() capabilities.Session {
	return c.session
}

func (c *runtimeCtx) Input() capabilities.Editor {
	return c.input
}

func (c *runtimeCtx) Selection() string {
	return c.selection
}

func (c *runtimeCtx) Send(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if c.prompts == nil {
		return
	}
	c.prompts <- text
}

func (c *runtimeCtx) Chat(role, text string) {
	c.enqueue(func(state *State) {
		state.AddMessage(role, text)
	})
}

func (c *runtimeCtx) Copy(text string) {
	if text == "" {
		return
	}
	if err := c.Services().Clipboard().Copy(text); err != nil {
		c.Chat("system", "copy failed: "+err.Error())
	}
}

// Services returns the host capability services bound to this runtime context.
// The Completion registry is shared across the build; Palette/Clipboard/Selection
// are bound to this context's state.
func (c *runtimeCtx) Services() capabilities.Services {
	registry := c.completion
	if registry == nil {
		registry = newCompletionRegistry()
	}
	return services{completion: registry, rt: c}
}

func (c *runtimeCtx) Shell(ctx context.Context, name string, args ...string) (string, error) {
	if c.shellTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.shellTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if c.workingDir != "" {
		cmd.Dir = c.workingDir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return string(out), ctx.Err()
		}
		return string(out), fmt.Errorf("%s: %w", name, err)
	}
	return string(out), nil
}

func (c *runtimeCtx) SetLayout(name string) {
	c.enqueue(func(state *State) {
		state.SetLayout(LayoutTypeFromString(name))
	})
}

func (c *runtimeCtx) OpenList(title string, items []capabilities.ListItem, onPick func(capabilities.ListItem)) {
	c.Services().Palette().Open(title, items, onPick)
}

// --- Command verbs ---
//
// Each verb enqueues the same effect the legacy PaletteAction handler ran. The
// pure-state toggles mutate state directly through a mutation; the
// backend-dependent verbs set PendingAction so the run loop's dispatch
// (handlePaletteAction) executes them with the live backend client. This keeps
// the existing internal behavior intact ahead of the Phase 6 backend refactor.

func (c *runtimeCtx) OpenCommandPalette() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionOpenPalette })
}

func (c *runtimeCtx) ChangeModel() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionChangeModel })
}

func (c *runtimeCtx) SwitchBackend() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionBackendList })
}

func (c *runtimeCtx) NewSession() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionNewSession })
}

func (c *runtimeCtx) OpenSessions() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionSessionsList })
}

func (c *runtimeCtx) OpenSkills() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionSkillsList })
}

func (c *runtimeCtx) SwitchMode() {
	c.enqueue(func(state *State) { state.ToggleMode() })
}

func (c *runtimeCtx) CycleThinkingVisibility() {
	c.enqueue(func(state *State) { state.CycleThinkingVisibility() })
}

func (c *runtimeCtx) CycleToolCallVisibility() {
	c.enqueue(func(state *State) { state.CycleToolCallVisibility() })
}

func (c *runtimeCtx) AdjustOutputPercent(delta int) {
	c.enqueue(func(state *State) { state.AdjustOutputPercentage(delta) })
}

func (c *runtimeCtx) SwitchLayout() {
	c.enqueue(func(state *State) { state.PendingAction = PaletteActionLayoutsList })
}

func (c *runtimeCtx) State() capabilities.CommandState {
	return c.commandState
}

func (c *runtimeCtx) enqueue(mutation stateMutation) {
	if mutation == nil || c.mutations == nil {
		return
	}
	c.mutations <- mutation
}

type runtimeEditor struct {
	text      string
	mutations chan<- stateMutation
}

func (e *runtimeEditor) Text() string {
	return e.text
}

func (e *runtimeEditor) SetText(text string) {
	e.text = text
	if e.mutations == nil {
		return
	}
	e.mutations <- func(state *State) {
		state.Editor.SetContent(text)
	}
}
