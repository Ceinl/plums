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

	root, err := buildLayout(state, cfg, layoutName(state.EffectiveLayout()))
	if err != nil {
		root = CreateDefaultLayout(state)
	}

	root.Layout(0, 0, state.width, state.height)
	root.Render(scr)
	renderFullscreenTabs(state)
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
	if !state.FullscreenShowsEditor() {
		return
	}
	cx, cy := state.Editor.CursorScreenPos()
	scr.SetCursor(cx, cy)
	scr.ShowCursor()
}

func renderFullscreenTabs(state *State) {
	if state.EffectiveLayout() != LayoutFullscreen || state.width < 20 || state.height < 1 {
		return
	}

	y := 0
	if state.height > 2 {
		y = 1
	}

	bg := theme.BgBase.Bg()
	fg := theme.TextMuted.Fg()
	activeBg := theme.Accent.Bg()
	activeFg := theme.BgBase.Fg()
	sideFg := theme.TextSoft.Fg()

	for col := 0; col < state.width; col++ {
		scr.Set(col, y, ' ', fg, bg, "")
	}

	tabs := []struct {
		label string
		tab   FullscreenTab
	}{
		{"editor", FullscreenTabEditor},
		{"output", FullscreenTabOutput},
		{"diff", FullscreenTabDiff},
	}
	labels := make([]string, len(tabs))
	totalW := 0
	for i, tab := range tabs {
		labels[i] = " " + tab.label + " "
		totalW += len(labels[i])
	}
	totalW += len(tabs) - 1
	tabsX := (state.width - totalW) / 2
	if tabsX < 0 {
		tabsX = 0
	}
	tabsEnd := tabsX + totalW

	drawText := func(x int, text string, fgColor, bgColor string) {
		for _, r := range text {
			if x < 0 {
				x++
				continue
			}
			if x >= state.width {
				return
			}
			scr.Set(x, y, r, fgColor, bgColor, "")
			x++
		}
	}

	x := tabsX
	writeTab := func(text string, fgColor, bgColor string) {
		for _, r := range text {
			if x >= state.width {
				return
			}
			scr.Set(x, y, r, fgColor, bgColor, "")
			x++
		}
	}

	left := truncateRunes(fullscreenModelLabel(state), tabsX-3)
	if left != "" {
		drawText(2, left, sideFg, bg)
	}

	right := fullscreenSessionLabel(state)
	rightMax := state.width - tabsEnd - 3
	right = truncateRunes(right, rightMax)
	if right != "" {
		drawText(state.width-2-len([]rune(right)), right, sideFg, bg)
	}

	for i, tab := range tabs {
		if i > 0 {
			writeTab(" ", fg, bg)
		}
		if state.FullscreenTab == tab.tab {
			writeTab(labels[i], activeFg, activeBg)
		} else {
			writeTab(labels[i], fg, bg)
		}
	}
}

func fullscreenModelLabel(state *State) string {
	if state.ModelProvider != "" && state.ModelID != "" {
		return state.ModelProvider + "/" + state.ModelID
	}
	if state.ModelID != "" {
		return state.ModelID
	}
	return "no model"
}

func fullscreenSessionLabel(state *State) string {
	if state.SessionTitle != "" {
		return state.SessionTitle
	}
	if state.SessionID != "" {
		return state.SessionID
	}
	return "no session"
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "~"
}

func layoutName(t LayoutType) string {
	switch t {
	case LayoutChat:
		return "chat"
	case LayoutSplit:
		return "split"
	case LayoutFullscreen:
		return "fullscreen"
	default:
		return "chat"
	}
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
