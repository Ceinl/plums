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
	popup := c.widget()
	popup.SetTitle(rctx.PaletteTitle())
	popup.SetQuery(rctx.PaletteQuery())
	items := rctx.PaletteItems()
	popupItems := make([]components.PopupItem, len(items))
	for i, item := range items {
		popupItems[i] = components.PopupItem{Title: item.Title, Detail: item.Detail, Disabled: item.Disabled}
	}
	popup.SetItems(popupItems, rctx.PaletteIndex())
	popup.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	popup.Render(scr)
}
