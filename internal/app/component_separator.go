package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// separatorComponent is the public status_separator / vertical_status_separator
// pane. The Separator widget picks the horizontal vs vertical glyph from its
// arranged dimensions, so both registered names share one implementation.
type separatorComponent struct {
	name string
	sep  *components.Separator
	rect capabilities.Rect
}

func NewStatusSeparatorComponent() capabilities.Component {
	return &separatorComponent{name: "status_separator"}
}

func NewVerticalStatusSeparatorComponent() capabilities.Component {
	return &separatorComponent{name: "vertical_status_separator"}
}

func (c *separatorComponent) Name() string { return c.name }

func (c *separatorComponent) NewComponent() capabilities.Component {
	return &separatorComponent{name: c.name, sep: components.NewSeparator()}
}

func (c *separatorComponent) widget() *components.Separator {
	if c.sep == nil {
		c.sep = components.NewSeparator()
	}
	return c.sep
}

func (c *separatorComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *separatorComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	sep := c.widget()
	sep.SetStatus(rctx.ServerStarting(), rctx.ServerReady(), rctx.Streaming())
	sep.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	sep.Render(scr)
}
