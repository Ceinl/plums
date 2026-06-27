package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// diffLogComponent is the public difflog pane: a standalone git diff viewer. It
// renders the State-owned DiffLog so the central scroll/selection routing keeps
// targeting the same instance.
type diffLogComponent struct {
	rect  capabilities.Rect
	state *State
}

var _ capabilities.Scrollable = (*diffLogComponent)(nil)

func NewDiffLogComponent() capabilities.Component {
	return &diffLogComponent{}
}

func (c *diffLogComponent) Name() string { return "difflog" }

func (c *diffLogComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *diffLogComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	provider, ok := rctx.(appStateProvider)
	if !ok {
		return
	}
	state := provider.appState()
	if state == nil {
		return
	}
	c.state = state
	diff := state.DiffLog()
	diff.SetContent(rctx.GitDiff())
	diff.SetScrollOffset(state.OutputScroll())
	diff.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	diff.Render(scr)
}

func (c *diffLogComponent) Scroll(delta int) bool {
	if c.state == nil {
		return false
	}
	return c.state.scrollOutputOffset(delta)
}

func (c *diffLogComponent) ScrollToBottom() bool {
	if c.state == nil {
		return false
	}
	return c.state.scrollOutputOffsetBottom()
}
