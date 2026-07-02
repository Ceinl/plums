package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/plugins/layouts"
	layout "github.com/Ceinl/plums/plums/layout"
)

// testRenderConfig builds a RenderConfig the way the runtime does: start from the
// internal scaffold (overlays only) and install the public layouts from the
// bundled layouts plugin, plus the split layout standing in for the user-config
// SplitLayoutPlugin (a compiled template, not linked into tests). This gives
// tests the same zen/split layout set the app sees at runtime, without embedded
// layout data files.
func testRenderConfig(t *testing.T) *RenderConfig {
	t.Helper()
	cfg := NewRenderConfig()
	all := append([]capabilities.Layout{}, (&layouts.Plugin{}).Layouts()...)
	all = append(all,
		layout.Named("split", testSplitTree()),
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
			Style(layout.ThemeStyle(layout.ColorBgPanel, layout.ColorText)).
			WhenPopupOpen(
				layout.Component("command_palette_panel").Padding(testPad(1, 2, 1, 2)),
			),
		layout.VerticalSeparator().Width(1).Height("100%"),
		layout.Column(
			layout.Tabs().Width("100%").Height(1).Style(layout.ThemeBg(layout.ColorBgBase)),
			layout.InfoView().Variants(map[string]string{
				"ai":       "chat_log",
				"git_diff": "git_diff_log",
			}),
			layout.SplitStatusBar().Width("100%").Height(1).Style(layout.ThemeStyle(layout.ColorBgBase, layout.ColorTextFaint)),
		).
			Width("grow").Height("100%").Padding(testPad(1, 2, 1, 2)).Style(layout.ThemeBg(layout.ColorBgBase)),
	).
		Width("100%").Height("100%")
}

func testPad(top, right, bottom, left float64) layout.Padding {
	return layout.Padding{Top: &top, Right: &right, Bottom: &bottom, Left: &left}
}
