package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/plugins/layouts"
	layout "github.com/Ceinl/plums/plums/layout"
)

// testRenderConfig builds a RenderConfig the way the runtime does: start from the
// internal scaffold (overlays only) and install the public layouts from the
// bundled layouts plugin, plus the split layouts standing in for the user-config
// SplitLayoutPlugin (a compiled template, not linked into tests). This gives
// tests the same chat/zen/narrow_chat/split/narrow_split set the app sees at
// runtime, without an embedded layout.json.
func testRenderConfig(t *testing.T) *RenderConfig {
	t.Helper()
	cfg := NewRenderConfig()
	all := append([]capabilities.Layout{}, (&layouts.Plugin{}).Layouts()...)
	all = append(all,
		layout.Named("split", testSplitTree()),
		layout.Hidden(layout.Named("narrow_split", testNarrowSplitTree())),
	)
	for _, l := range all {
		if _, err := InstallPublicLayout(cfg, l); err != nil {
			t.Fatalf("install layout %s: %v", l.Name(), err)
		}
	}
	return cfg
}

func testSplitTree() capabilities.Node {
	return layout.Row(
		layout.EditorOrPalette().
			Width("state.SplitLeftPercent%").
			Height("100%").
			Padding(testPad(1, 2, 1, 2)).
			Style(testBgFg(32, 30, 40, 232, 229, 241)).
			WhenPopupOpen(
				layout.Component("command_palette_panel").Padding(testPad(1, 2, 1, 2)),
			),
		layout.VerticalSeparator().Width(1).Height("100%"),
		layout.Column(
			layout.Tabs().Width("100%").Height(1).Style(testBg(22, 20, 27)),
			layout.InfoView().Variants(map[string]string{
				"ai":       "chat_log",
				"git_diff": "git_diff_log",
			}),
			layout.SplitStatusBar().Width("100%").Height(1).Style(testBgFg(22, 20, 27, 100, 98, 112)),
		).
			Width("grow").Height("100%").Padding(testPad(1, 2, 1, 2)).Style(testBg(22, 20, 27)),
	).
		Width("100%").Height("100%").MinWidth("MinSplitLayoutWidth").Fallback("narrow_split")
}

func testNarrowSplitTree() capabilities.Node {
	return layout.Column(
		layout.Chat().Width("100%").Height("50%").Padding(testPad(1, 2, 1, 2)).Style(testBg(22, 20, 27)),
		layout.Separator().Width("100%").Height(1),
		layout.Editor().Width("100%").Height("grow").Padding(testPad(1, 2, 1, 2)).Style(testBgFg(32, 30, 40, 232, 229, 241)),
	).
		Width("100%").Height("100%")
}

func testPad(top, right, bottom, left float64) layout.Padding {
	return layout.Padding{Top: &top, Right: &right, Bottom: &bottom, Left: &left}
}

func testBg(r, g, b uint8) layout.Style { return layout.Style{Background: []uint8{r, g, b}} }

func testBgFg(br, bgc, bb, fr, fg, fb uint8) layout.Style {
	return layout.Style{Background: []uint8{br, bgc, bb}, Foreground: []uint8{fr, fg, fb}}
}
