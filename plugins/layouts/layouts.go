// Package layouts is the built-in layout plugin. It ships only the minimal
// stock Zen layout as a public, forkable LayoutProvider plugin wired by the
// Default Config. Other layouts, including the default split layout seeded into
// the user's global config, prove that users can own layout arrangement through
// the same registry path as bundled plugins.
package layouts

import (
	"github.com/Ceinl/plums/capabilities"
	layout "github.com/Ceinl/plums/plums/layout"
)

// Plugin contributes the only bundled layout. `split` deliberately ships NOT
// here but as a user plugin — see the default config.go template — so the
// default two-pane experience flows through the kernel registry exactly like a
// user-authored override.
type Plugin struct{}

func (*Plugin) Name() string                      { return "ui/layouts" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) Layouts() []capabilities.Layout {
	return []capabilities.Layout{
		layout.Named("zen", zenLayout()),
	}
}

func zenLayout() capabilities.Node {
	return layout.Column(
		layout.Chat().
			Width("100%").
			Height("grow").
			Padding(padding(2, 6, 1, 6)).
			Style(layout.ThemeBg(layout.ColorBgBase)),
		layout.InputBox().
			Width("100%").
			Height(7).
			Padding(padding(0, 6, 2, 6)).
			Style(layout.ThemeStyle(layout.ColorBgBase, layout.ColorText)),
	).
		Width("100%").
		Height("100%").
		AlignItems("center").
		Style(layout.ThemeBg(layout.ColorBgBase))
}

func padding(top, right, bottom, left float64) layout.Padding {
	return layout.Padding{
		Top:    &top,
		Right:  &right,
		Bottom: &bottom,
		Left:   &left,
	}
}
