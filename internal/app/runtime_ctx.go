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
type runtimeAction func()

type runtimeActions struct {
	changeModel    func()
	setModel       func(providerID, modelID string)
	switchBackend  func()
	selectBackend  func(id string)
	newSession     func()
	openSessions   func()
	openSession    func(id string)
	openSkills     func()
	answerQuestion func(answer string)
	switchLayout   func()
}

type runtimeCtx struct {
	session      capabilities.Session
	input        *runtimeEditor
	selection    string
	workingDir   string
	shellTimeout time.Duration
	clipboardCmd string
	mutations    chan<- stateMutation
	actions      chan<- runtimeAction
	actionFns    runtimeActions
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
// dynamic palette labels.
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

// --- Grouped host capabilities ---
//
// View/Backends/Sessions namespace the host actions a command can trigger. Pure-
// state toggles mutate state directly; backend/session/model actions enqueue
// runtime actions so the event loop runs them with the live backend client.

func (c *runtimeCtx) View() capabilities.View         { return ctxView{c} }
func (c *runtimeCtx) Backends() capabilities.Backends { return ctxBackends{c} }
func (c *runtimeCtx) Sessions() capabilities.Sessions { return ctxSessions{c} }

type ctxView struct{ c *runtimeCtx }

func (v ctxView) SwitchMode() {
	v.c.enqueue(func(state *State) { state.ToggleMode() })
}

func (v ctxView) CycleThinkingVisibility() {
	v.c.enqueue(func(state *State) { state.CycleThinkingVisibility() })
}

func (v ctxView) CycleToolCallVisibility() {
	v.c.enqueue(func(state *State) { state.CycleToolCallVisibility() })
}

func (v ctxView) SwitchLayout() {
	v.c.enqueueAction(v.c.actionFns.switchLayout)
}

func (v ctxView) OpenSkills() {
	v.c.enqueueAction(v.c.actionFns.openSkills)
}

type ctxBackends struct{ c *runtimeCtx }

func (b ctxBackends) Switch() {
	b.c.enqueueAction(b.c.actionFns.switchBackend)
}

func (b ctxBackends) Select(id string) {
	b.c.enqueueAction(func() {
		if b.c.actionFns.selectBackend != nil {
			b.c.actionFns.selectBackend(id)
		}
	})
}

func (b ctxBackends) ChangeModel() {
	b.c.enqueueAction(b.c.actionFns.changeModel)
}

func (b ctxBackends) SetModel(providerID, modelID string) {
	b.c.enqueueAction(func() {
		if b.c.actionFns.setModel != nil {
			b.c.actionFns.setModel(providerID, modelID)
		}
	})
}

func (b ctxBackends) AnswerQuestion(answer string) {
	b.c.enqueueAction(func() {
		if b.c.actionFns.answerQuestion != nil {
			b.c.actionFns.answerQuestion(answer)
		}
	})
}

type ctxSessions struct{ c *runtimeCtx }

func (s ctxSessions) New() {
	s.c.enqueueAction(s.c.actionFns.newSession)
}

func (s ctxSessions) Picker() {
	s.c.enqueueAction(s.c.actionFns.openSessions)
}

func (s ctxSessions) Open(id string) {
	s.c.enqueueAction(func() {
		if s.c.actionFns.openSession != nil {
			s.c.actionFns.openSession(id)
		}
	})
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

func (c *runtimeCtx) enqueueAction(action runtimeAction) {
	if action == nil || c.actions == nil {
		return
	}
	c.actions <- action
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
