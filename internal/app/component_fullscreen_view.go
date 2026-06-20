package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// fullscreenViewComponent is the public fullscreen_view pane. It builds the
// State-owned fullscreen widget tree (editor pane or chat/diff output) and
// renders it, reusing the same State-owned widgets the central routing targets.
//
// Phase 3 removes this component along with the fullscreen layout; for now it is
// ported so the privileged factory path can be deleted entirely.
type fullscreenViewComponent struct {
	rect capabilities.Rect
}

func NewFullscreenViewComponent() capabilities.Component {
	return &fullscreenViewComponent{}
}

func (c *fullscreenViewComponent) Name() string { return "fullscreen_view" }

func (c *fullscreenViewComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *fullscreenViewComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
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
	view := newFullscreenView(state)
	view.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	view.Render(scr)
}
