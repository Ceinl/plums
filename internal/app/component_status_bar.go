package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// statusBarComponent is the public status_bar / split_status_bar pane. It is a
// pure display surface: it reads server/session/mode/model state from RenderCtx
// and draws the existing StatusBar widget. showSession distinguishes the two
// registered variants.
type statusBarComponent struct {
	name        string
	showSession bool
	bar         *components.StatusBar
	rect        capabilities.Rect
}

func NewStatusBarComponent() capabilities.Component {
	return &statusBarComponent{name: "status_bar", showSession: false}
}

func NewSplitStatusBarComponent() capabilities.Component {
	return &statusBarComponent{name: "split_status_bar", showSession: true}
}

func (c *statusBarComponent) Name() string { return c.name }

func (c *statusBarComponent) NewComponent() capabilities.Component {
	return &statusBarComponent{name: c.name, showSession: c.showSession, bar: components.NewStatusBar()}
}

func (c *statusBarComponent) widget() *components.StatusBar {
	if c.bar == nil {
		c.bar = components.NewStatusBar()
	}
	return c.bar
}

func (c *statusBarComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *statusBarComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	bar := c.widget()
	starting, ready, mode := false, false, ""
	if sv, ok := rctx.(capabilities.ServerStatusView); ok {
		starting, ready, mode = sv.ServerStarting(), sv.ServerReady(), sv.Mode()
	}
	bar.SetStatus(starting, ready, rctx.Streaming())
	bar.SetSession(rctx.Session().Title)
	bar.SetMode(mode)
	if model := rctx.Session().Model; model != nil {
		bar.SetModel(model.ProviderID, model.ID)
	} else {
		bar.SetModel("", "")
	}
	bar.SetShowSession(c.showSession)
	bar.SetStyle(bgStyle(rctx.Background()))
	bar.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	bar.Render(scr)
}
