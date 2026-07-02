package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// sessionsComponent is the public sessions pane (vertical sidebar list or the
// horizontal narrow-layout tab strip). It renders the State-owned Sessions
// widget so the central click/scroll routing in keyboard.go keeps targeting the
// same instance — display data flows in through RenderCtx.
type sessionsComponent struct {
	name        string
	orientation components.SessionsOrientation
	rect        capabilities.Rect
	state       *State
}

func NewSessionsComponent() capabilities.Component {
	return &sessionsComponent{name: "sessions", orientation: components.SessionsVertical}
}

func NewSessionsVerticalComponent() capabilities.Component {
	return &sessionsComponent{name: "sessions_vertical", orientation: components.SessionsVertical}
}

func NewSessionsHorizontalComponent() capabilities.Component {
	return &sessionsComponent{name: "sessions_horizontal", orientation: components.SessionsHorizontal}
}

func (c *sessionsComponent) Name() string { return c.name }

func (c *sessionsComponent) NewComponent() capabilities.Component {
	return &sessionsComponent{name: c.name, orientation: c.orientation}
}

func (c *sessionsComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *sessionsComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
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

	sessions := state.Sessions()
	if c.orientation == components.SessionsHorizontal {
		sessions = state.SessionsHorizontal()
	}
	sessions.SetOrientation(c.orientation)

	var items []capabilities.SessionItem
	if sv, ok := rctx.(capabilities.SessionsView); ok {
		items = sv.Sessions()
	}
	widgetItems := make([]components.SessionItem, len(items))
	for i, item := range items {
		widgetItems[i] = components.SessionItem{
			ID:        item.ID,
			Title:     item.Title,
			Directory: item.Directory,
			Updated:   item.Updated,
			Current:   item.Current,
		}
	}
	sessions.SetItems(widgetItems)
	sessions.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	sessions.Render(scr)
}

func (c *sessionsComponent) HandleMouse(ctx capabilities.Ctx, ev capabilities.MouseEvent) bool {
	if c.state == nil || ev.Action != capabilities.MousePress {
		return false
	}
	return c.state.SessionMouseDown(ctx, ev.X, ev.Y)
}

func (c *sessionsComponent) Scroll(delta int) bool {
	if c.state == nil {
		return false
	}
	if c.orientation == components.SessionsHorizontal {
		return c.state.SessionsHorizontal().Scroll(delta)
	}
	return c.state.Sessions().Scroll(delta)
}
