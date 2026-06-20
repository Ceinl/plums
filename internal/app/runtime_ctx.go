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
	if err := writeClipboard(text, c.clipboardCmd); err != nil {
		c.Chat("system", "copy failed: "+err.Error())
	}
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
	items = append([]capabilities.ListItem(nil), items...)
	c.enqueue(func(state *State) {
		state.SetRuntimeList(title, items, onPick)
	})
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
