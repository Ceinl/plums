package app

import (
	"strings"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

func ComponentFactoryForPublic(component capabilities.Component) ComponentFactory {
	return func(state *State, node LayoutNode) (layout.Component, error) {
		instance := component
		// A template component produces a fresh, build-cached instance so it can
		// own private render/input state (selection, scroll) across rebuilds. The
		// cache is keyed by layout slot, so the same component used in two slots
		// gets independent state.
		if instancer, ok := component.(capabilities.ComponentInstancer); ok {
			if state != nil {
				key := component.Name() + "@" + node.slotID
				instance = state.publicComponentInstance(key, instancer.NewComponent)
			} else {
				instance = instancer.NewComponent()
			}
		}
		adapter := &publicComponentAdapter{
			state:     state,
			component: instance,
		}
		if state != nil {
			state.addPublicComponent(adapter)
		}
		return adapter, nil
	}
}

type publicComponentAdapter struct {
	state     *State
	component capabilities.Component
	rect      capabilities.Rect
	parent    layout.Component
	dirty     bool
}

func (a *publicComponentAdapter) GetStyle() layout.Style {
	return layout.Style{}
}

func (a *publicComponentAdapter) IsDirty() bool {
	if tracker, ok := a.component.(capabilities.DirtyTracker); ok && tracker.IsDirty() {
		return true
	}
	return a.dirty
}

func (a *publicComponentAdapter) MakeDirty() {
	a.dirty = true
}

func (a *publicComponentAdapter) ClearDirty() {
	a.dirty = false
	if tracker, ok := a.component.(capabilities.DirtyTracker); ok {
		tracker.ClearDirty()
	}
}

func (a *publicComponentAdapter) SetParent(parent layout.Component) {
	a.parent = parent
}

func (a *publicComponentAdapter) Layout(x, y, w, h int) {
	a.rect = capabilities.Rect{X: x, Y: y, W: w, H: h}
	if a.component != nil {
		a.component.Arrange(a.rect)
	}
	a.dirty = true
}

func (a *publicComponentAdapter) Render(scr *screen.Screen) {
	if a.component == nil {
		return
	}
	a.component.Render(renderCtx{state: a.state, rect: a.rect, background: a.background()}, scr)
}

// background resolves the slot's themed background from the wrapping layout div
// (the same source the internal layout engine uses), falling back to the base
// theme background so a pane never paints pure black over the layout.
func (a *publicComponentAdapter) background() string {
	if a.parent != nil {
		if bg := a.parent.GetStyle().GetBackground(); bg != "" {
			return bg
		}
	}
	return theme.BgBase.Bg()
}

func (a *publicComponentAdapter) Selection() string {
	if provider, ok := a.component.(capabilities.SelectionProvider); ok {
		return provider.Selection()
	}
	return ""
}

func (a *publicComponentAdapter) contains(x, y int) bool {
	return x >= a.rect.X && x < a.rect.X+a.rect.W && y >= a.rect.Y && y < a.rect.Y+a.rect.H
}

func HandlePublicComponentEvent(state *State, ev keyboard.Event, ctx capabilities.Ctx) bool {
	if state == nil || len(state.publicComponents) == 0 {
		return false
	}
	if handled := handlePublicComponentMouse(state, ev, ctx); handled {
		return true
	}
	return handlePublicComponentKey(state, ev, ctx)
}

func handlePublicComponentKey(state *State, ev keyboard.Event, ctx capabilities.Ctx) bool {
	event, ok := publicKeyEvent(ev)
	if !ok {
		return false
	}
	for i := len(state.publicComponents) - 1; i >= 0; i-- {
		handler, ok := state.publicComponents[i].component.(capabilities.KeyHandler)
		if !ok {
			continue
		}
		if handler.HandleKey(ctx, event) {
			return true
		}
	}
	return false
}

func handlePublicComponentMouse(state *State, ev keyboard.Event, ctx capabilities.Ctx) bool {
	event, ok := publicMouseEvent(ev)
	if !ok {
		return false
	}
	// While a drag is in progress, route every event to the component that
	// captured the press so selection continues past the pane's bounds.
	if state.mouseCapture != nil {
		if handler, ok := state.mouseCapture.component.(capabilities.MouseHandler); ok {
			handler.HandleMouse(ctx, event)
		}
		if event.Action == capabilities.MouseRelease {
			state.mouseCapture = nil
		}
		return true
	}
	for i := len(state.publicComponents) - 1; i >= 0; i-- {
		component := state.publicComponents[i]
		if !component.contains(event.X, event.Y) {
			continue
		}
		handler, ok := component.component.(capabilities.MouseHandler)
		if !ok {
			continue
		}
		if handler.HandleMouse(ctx, event) {
			if event.Action == capabilities.MousePress {
				state.mouseCapture = component
			}
			return true
		}
	}
	return false
}

func publicKeyEvent(ev keyboard.Event) (capabilities.KeyEvent, bool) {
	ev = normalizeKeybindEvent(ev)
	key := publicKeyName(ev)
	if key == "" {
		return capabilities.KeyEvent{}, false
	}
	return capabilities.KeyEvent{
		Key:   key,
		Rune:  ev.Ch,
		Ctrl:  ev.Ctrl,
		Alt:   ev.Alt,
		Shift: ev.Shift,
		Cmd:   ev.Cmd,
	}, true
}

func publicKeyName(ev keyboard.Event) string {
	switch ev.Type {
	case keyboard.KeyRune:
		if ev.Ch == 0 {
			return ""
		}
		return strings.ToLower(string(ev.Ch))
	case keyboard.KeyEnter:
		return "enter"
	case keyboard.KeyBackspace:
		return "backspace"
	case keyboard.KeyTab:
		return "tab"
	case keyboard.KeyEscape:
		return "escape"
	case keyboard.KeyArrowUp:
		return "up"
	case keyboard.KeyArrowDown:
		return "down"
	case keyboard.KeyArrowRight:
		return "right"
	case keyboard.KeyArrowLeft:
		return "left"
	case keyboard.KeyHome:
		return "home"
	case keyboard.KeyEnd:
		return "end"
	case keyboard.KeyPageUp:
		return "pageup"
	case keyboard.KeyPageDown:
		return "pagedown"
	case keyboard.KeyDelete:
		return "delete"
	default:
		return ""
	}
}

func publicMouseEvent(ev keyboard.Event) (capabilities.MouseEvent, bool) {
	event := capabilities.MouseEvent{X: ev.MouseX, Y: ev.MouseY}
	switch ev.Type {
	case keyboard.KeyMouseLeftDown:
		event.Button = capabilities.MouseButtonLeft
		event.Action = capabilities.MousePress
	case keyboard.KeyMouseLeftDrag:
		event.Button = capabilities.MouseButtonLeft
		event.Action = capabilities.MouseDrag
	case keyboard.KeyMouseLeftUp:
		event.Button = capabilities.MouseButtonLeft
		event.Action = capabilities.MouseRelease
	default:
		return capabilities.MouseEvent{}, false
	}
	return event, true
}

type renderCtx struct {
	state      *State
	rect       capabilities.Rect
	background string
}

// appState exposes the in-tree *State to ported display components that reuse a
// State-owned widget instance (chat log, diff log, sessions). Reusing the shared
// instance keeps the central mouse/scroll routing in keyboard.go targeting the
// same widget the public component renders, so behavior is unchanged. Only
// app-package components can reach this; external public components see the
// RenderCtx accessors only.
type appStateProvider interface {
	appState() *State
}

func (c renderCtx) appState() *State { return c.state }

func (c renderCtx) Rect() capabilities.Rect {
	return c.rect
}

func (c renderCtx) Background() string {
	return c.background
}

func (c renderCtx) Theme() capabilities.Theme {
	if c.state != nil {
		return c.state.EffectiveTheme()
	}
	return capabilities.Theme{Name: "default"}
}

func (c renderCtx) Messages() []capabilities.Message {
	if c.state == nil {
		return nil
	}
	messages := c.state.Messages()
	out := make([]capabilities.Message, 0, len(messages))
	for _, message := range messages {
		out = append(out, capabilities.Message{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return out
}

func (c renderCtx) Streaming() bool {
	return c.state != nil && c.state.IsStreaming()
}

func (c renderCtx) StreamingText() string {
	if c.state == nil {
		return ""
	}
	return c.state.aioutput
}

func (c renderCtx) ThinkingVisibility() int {
	if c.state == nil {
		return 0
	}
	return int(c.state.ThinkingMode)
}

func (c renderCtx) ToolCallVisibility() int {
	if c.state == nil {
		return 0
	}
	return int(c.state.ToolCallMode)
}

func (c renderCtx) Session() capabilities.Session {
	if c.state == nil {
		return capabilities.Session{}
	}
	session := capabilities.Session{
		ID:    c.state.SessionID,
		Title: c.state.SessionTitle,
	}
	if c.state.ModelID != "" || c.state.ModelProvider != "" {
		session.Model = &capabilities.ModelRef{
			ID:         c.state.ModelID,
			ProviderID: c.state.ModelProvider,
		}
	}
	return session
}

func (c renderCtx) Input() capabilities.EditorView {
	text := ""
	if c.state != nil && c.state.Editor != nil {
		text = c.state.Editor.GetContent()
	}
	return renderEditorView{text: text}
}

type renderEditorView struct {
	text string
}

func (v renderEditorView) Text() string {
	return v.text
}

func (c renderCtx) ServerStarting() bool {
	return c.state != nil && c.state.ServerStarting
}

func (c renderCtx) ServerReady() bool {
	return c.state != nil && c.state.ServerReady
}

func (c renderCtx) Mode() string {
	if c.state == nil {
		return ""
	}
	return c.state.Mode
}

func (c renderCtx) SpinnerFrame() int {
	if c.state == nil {
		return 0
	}
	return c.state.spinnerFrame
}

func (c renderCtx) Sessions() []capabilities.SessionItem {
	if c.state == nil {
		return nil
	}
	items := make([]capabilities.SessionItem, 0, len(c.state.SessionItems)+1)
	foundCurrent := false
	for _, item := range c.state.SessionItems {
		current := item.Current || item.ID == c.state.SessionID
		if current {
			foundCurrent = true
		}
		items = append(items, capabilities.SessionItem{
			ID:        item.ID,
			Title:     item.Title,
			Directory: item.Directory,
			Updated:   item.Updated,
			Current:   current,
		})
	}
	if !foundCurrent && (c.state.SessionID != "" || c.state.SessionTitle != "") {
		items = append([]capabilities.SessionItem{{
			ID:      c.state.SessionID,
			Title:   c.state.SessionTitle,
			Current: true,
		}}, items...)
	}
	return items
}

func (c renderCtx) SelectedSession() string {
	if c.state == nil {
		return ""
	}
	return c.state.SessionID
}

func (c renderCtx) InfoView() string {
	if c.state != nil && c.state.InfoView == InfoViewGitDiff {
		return "git_diff"
	}
	return "chat"
}

func (c renderCtx) InfoTabs() []capabilities.InfoTabItem {
	active := c.InfoView()
	return []capabilities.InfoTabItem{
		{Label: "AI output", Active: active == "chat"},
		{Label: "Git diff", Active: active == "git_diff"},
	}
}

func (c renderCtx) GitDiff() string {
	if c.state == nil {
		return ""
	}
	return c.state.GitDiff
}

func (c renderCtx) SplitLeftPercent() int {
	if c.state == nil {
		return 0
	}
	return c.state.SplitLeftPercent()
}

func (c renderCtx) OutputPercent() int {
	if c.state == nil {
		return 0
	}
	return c.state.SplitOutputPercent()
}

func (c renderCtx) PaletteTitle() string {
	if c.state == nil {
		return ""
	}
	return c.state.PaletteTitle()
}

func (c renderCtx) PaletteQuery() string {
	if c.state == nil {
		return ""
	}
	return c.state.PaletteSearch()
}

func (c renderCtx) PaletteItems() []capabilities.PaletteItem {
	if c.state == nil {
		return nil
	}
	popupItems := c.state.PaletteItems()
	items := make([]capabilities.PaletteItem, len(popupItems))
	for i, item := range popupItems {
		items[i] = capabilities.PaletteItem{
			Title:    item.Title,
			Detail:   item.Detail,
			Disabled: item.Disabled,
		}
	}
	return items
}

func (c renderCtx) PaletteIndex() int {
	if c.state == nil {
		return 0
	}
	return c.state.PaletteIndex
}
