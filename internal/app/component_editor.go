package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// editorComponent is the public editor pane. It renders the State-owned editor
// widget (state.Editor) and owns the editor-local key handling while paste and
// mouse events still route through the app fallback.
type editorComponent struct {
	rect  capabilities.Rect
	state *State
}

func NewEditorComponent() capabilities.Component {
	return &editorComponent{}
}

func (c *editorComponent) Name() string { return "editor" }

func (c *editorComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *editorComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	provider, ok := rctx.(appStateProvider)
	if !ok {
		return
	}
	state := provider.appState()
	if state == nil || state.Editor == nil {
		return
	}
	c.state = state
	renderStateEditor(state.Editor, c.rect, scr)
}

func (c *editorComponent) HandleKey(ctx capabilities.Ctx, ev capabilities.KeyEvent) bool {
	if c.state != nil && c.state.PopupOpen {
		return handlePaletteKey(c.state, ev)
	}
	return handleEditorKey(c.state, ctx, ev)
}

// renderStateEditor draws the State-owned editor widget with the editor pane's
// themed background/foreground, mirroring the legacy bare-editor factory whose
// parent div carried the same BgPanel/Text style.
func renderStateEditor(ed *components.Editor, rect capabilities.Rect, scr *screen.Screen) {
	ed.SetParent(editorStyleParent{style: themedStyle(theme.BgPanel, theme.Text)})
	ed.Layout(rect.X, rect.Y, rect.W, rect.H)
	ed.Render(scr)
}

// editorStyleParent supplies the editor widget the same themed background and
// foreground its wrapping div provided on the legacy factory path. Only
// GetStyle is consulted by the editor renderer.
type editorStyleParent struct {
	style layout.Style
}

func (p editorStyleParent) GetStyle() layout.Style     { return p.style }
func (p editorStyleParent) IsDirty() bool              { return false }
func (p editorStyleParent) MakeDirty()                 {}
func (p editorStyleParent) ClearDirty()                {}
func (p editorStyleParent) Layout(x, y, w, h int)      {}
func (p editorStyleParent) Render(*screen.Screen)      {}
func (p editorStyleParent) SetParent(layout.Component) {}
