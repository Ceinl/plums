package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// inputBoxComponent is the public input_box (a.k.a. text_box) pane: the floating
// editor panel plus the coloured status line above it. It renders the
// State-owned editor widget (state.Editor) through an InputBox wrapper so the
// central key/mouse routing keeps targeting the same buffer instance.
type inputBoxComponent struct {
	name string
	box  *components.InputBox
	rect capabilities.Rect
}

func NewInputBoxComponent() capabilities.Component {
	return &inputBoxComponent{name: "input_box"}
}

func NewTextBoxComponent() capabilities.Component {
	return &inputBoxComponent{name: "text_box"}
}

func (c *inputBoxComponent) Name() string { return c.name }

func (c *inputBoxComponent) NewComponent() capabilities.Component {
	return &inputBoxComponent{name: c.name}
}

func (c *inputBoxComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *inputBoxComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
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

	if c.box == nil {
		c.box = components.NewInputBox(state.Editor)
	}
	c.box.SetParent(editorStyleParent{style: themedStyle(theme.BgBase, theme.Text)})
	c.box.SetStatusSegments(chatStatusSegments(state))
	c.box.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	c.box.Render(scr)
}
