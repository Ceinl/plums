package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// infoTabsComponent is the public info_tabs pane: the "AI output" / "Git diff"
// tab header above the split-layout info view.
type infoTabsComponent struct {
	tabs *components.InfoTabs
	rect capabilities.Rect
}

func NewInfoTabsComponent() capabilities.Component {
	return &infoTabsComponent{}
}

func (c *infoTabsComponent) Name() string { return "info_tabs" }

func (c *infoTabsComponent) NewComponent() capabilities.Component {
	return &infoTabsComponent{tabs: components.NewInfoTabs()}
}

func (c *infoTabsComponent) widget() *components.InfoTabs {
	if c.tabs == nil {
		c.tabs = components.NewInfoTabs()
	}
	return c.tabs
}

func (c *infoTabsComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *infoTabsComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	tabs := c.widget()
	var infoTabs []capabilities.InfoTabItem
	if info, ok := rctx.(capabilities.InfoPaneView); ok {
		infoTabs = info.InfoTabs()
	}
	widgetTabs := make([]components.InfoTab, len(infoTabs))
	for i, tab := range infoTabs {
		widgetTabs[i] = components.InfoTab{Label: tab.Label, Active: tab.Active}
	}
	tabs.SetTabs(widgetTabs)
	tabs.SetStyle(bgStyle(rctx.Background()))
	tabs.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	tabs.Render(scr)
}
