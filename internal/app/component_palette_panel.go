package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// palettePanelComponent is the public command_palette_panel pane: the inline
// command palette shown in the split layout's left column while the palette is
// open. It reads palette view state from RenderCtx and draws the Popup widget in
// panel mode.
type palettePanelComponent struct {
	popup *components.Popup
	rect  capabilities.Rect
	state *State
}

func NewCommandPalettePanelComponent() capabilities.Component {
	return &palettePanelComponent{}
}

func (c *palettePanelComponent) Name() string { return "command_palette_panel" }

func (c *palettePanelComponent) NewComponent() capabilities.Component {
	return &palettePanelComponent{popup: newPanelPopup()}
}

func newPanelPopup() *components.Popup {
	popup := components.NewPopup()
	popup.SetPanel(true)
	return popup
}

func (c *palettePanelComponent) widget() *components.Popup {
	if c.popup == nil {
		c.popup = newPanelPopup()
	}
	return c.popup
}

func (c *palettePanelComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *palettePanelComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	if provider, ok := rctx.(appStateProvider); ok {
		c.state = provider.appState()
	}
	pv, ok := rctx.(capabilities.PaletteView)
	if !ok {
		return
	}
	popup := c.widget()
	popup.SetTitle(pv.PaletteTitle())
	popup.SetQuery(pv.PaletteQuery())
	items := pv.PaletteItems()
	popupItems := make([]components.PopupItem, len(items))
	for i, item := range items {
		popupItems[i] = components.PopupItem{Title: item.Title, Detail: item.Detail, Disabled: item.Disabled}
	}
	popup.SetItems(popupItems, pv.PaletteIndex())
	popup.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	popup.Render(scr)
}

func (c *palettePanelComponent) HandleKey(ctx capabilities.Ctx, ev capabilities.KeyEvent) bool {
	return handlePaletteKey(c.state, ctx, ev)
}

func (c *palettePanelComponent) HandleMouse(_ capabilities.Ctx, ev capabilities.MouseEvent) bool {
	if c.state == nil || ev.Action != capabilities.MousePress {
		return false
	}
	c.state.ClosePalette()
	return true
}
