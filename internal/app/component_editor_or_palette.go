package app

import (
	"github.com/Ceinl/plums/capabilities"
)

// editorOrPaletteComponent is the public editor_or_palette pane used by the
// split layout's left column: it renders the editor normally, or the inline
// command-palette panel while the palette popup is open. This mirrors the
// layout's when_popup_open swap so behavior is unchanged.
type editorOrPaletteComponent struct {
	editor  capabilities.Component
	palette capabilities.Component
	rect    capabilities.Rect
}

func NewEditorOrPaletteComponent() capabilities.Component {
	return &editorOrPaletteComponent{
		editor:  NewEditorComponent(),
		palette: NewCommandPalettePanelComponent(),
	}
}

func (c *editorOrPaletteComponent) Name() string { return "editor_or_palette" }

func (c *editorOrPaletteComponent) NewComponent() capabilities.Component {
	return NewEditorOrPaletteComponent()
}

func (c *editorOrPaletteComponent) Arrange(rect capabilities.Rect) {
	c.rect = rect
	c.editor.Arrange(rect)
	c.palette.Arrange(rect)
}

func (c *editorOrPaletteComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	provider, ok := rctx.(appStateProvider)
	if ok {
		if state := provider.appState(); state != nil && state.PopupOpen {
			c.palette.Render(rctx, surface)
			return
		}
	}
	c.editor.Render(rctx, surface)
}
