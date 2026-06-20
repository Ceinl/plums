package app

import (
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

var scr *screen.Screen

// ── Main render entry point ───────────────────────────────────────────────────

func Render(state *State, cfg *RenderConfig) {
	if cfg == nil {
		cfg, _ = LoadRenderConfig("")
	}
	if scr == nil || scr.Width() != state.width || scr.Height() != state.height {
		scr = screen.NewScreen(state.width, state.height)
	}
	scr.Clear()

	// Swap in the active layout's palette (global Default unless the layout
	// overrides it) and refresh component colour caches so every surface —
	// chat, sessions, popups, status bar — recolours to match.
	theme.Apply(paletteForState(state))
	components.RefreshColors()

	state.BeginPublicComponentFrame()
	root, err := buildLayout(state, cfg, layoutName(state.EffectiveLayout()))
	if err != nil {
		state.BeginPublicComponentFrame()
		root = fallbackLayout(state)
	}

	root.Layout(0, 0, state.width, state.height)
	root.Render(scr)
	renderEditorDropdown(state, cfg)
	if overlayEnabled(state, cfg, "command_palette_popup") {
		popup := components.NewPopup()
		popup.SetTitle(state.PaletteTitle())
		popup.SetQuery(state.PaletteSearch())
		popup.SetItems(state.PaletteItems(), state.PaletteIndex)
		popup.Layout(0, 0, state.width, state.height)
		popup.Render(scr)
	}

	scr.Flush()
	if state.PopupOpen {
		return
	}
	cx, cy := state.Editor.CursorScreenPos()
	scr.SetCursor(cx, cy)
	scr.ShowCursor()
}

// paletteForState selects the active colour palette. An explicit configured
// theme wins; otherwise the current layout can select a layout-specific palette.
func paletteForState(state *State) theme.Palette {
	switch state.ThemeName() {
	case "default":
		return theme.Default
	case "zen":
		return theme.Zen
	default:
		return paletteForLayout(state.EffectiveLayout())
	}
}

// paletteForLayout maps a layout to its colour palette. Default is the global
// base; only layouts that want a distinct look override it.
func paletteForLayout(l LayoutType) theme.Palette {
	switch l {
	case LayoutZen:
		return theme.Zen
	default:
		return theme.Default
	}
}

// fallbackLayout picks a Go layout builder when the JSON config cannot be
// built, honouring the current layout type where a dedicated builder exists.
func fallbackLayout(state *State) *components.Div {
	switch state.EffectiveLayout() {
	case LayoutSplit:
		return CreateSplitLayout(state)
	case LayoutZen:
		return CreateZenLayout(state)
	default:
		return CreateDefaultLayout(state)
	}
}

// layoutName is the config key for a layout — identical to its id, except an
// empty id maps to the default layout.
func layoutName(t LayoutType) string {
	if t == "" {
		return string(LayoutDefault)
	}
	return string(t)
}

func drawOverlayText(x, y, maxW int, text, fg, bg string) {
	for i, r := range text {
		if i >= maxW {
			return
		}
		scr.Set(x+i, y, r, fg, bg, "")
	}
}

func drawOverlayFill(x, y, w int, bg string) {
	fg := theme.Text.Fg()
	for i := 0; i < w; i++ {
		scr.Set(x+i, y, ' ', fg, bg, "")
	}
}

// ansiFgColor and ansiBgColor resolve a user-configured RGB triple, falling
// back to the given theme colour when the config does not override it.
func ansiFgColor(c []uint8, fallback theme.Color) string {
	if len(c) == 3 {
		return theme.Color{R: c[0], G: c[1], B: c[2]}.Fg()
	}
	return fallback.Fg()
}

func ansiBgColor(c []uint8, fallback theme.Color) string {
	if len(c) == 3 {
		return theme.Color{R: c[0], G: c[1], B: c[2]}.Bg()
	}
	return fallback.Bg()
}
