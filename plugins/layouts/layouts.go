// Package layouts is the built-in layout plugin. It ships the standard plums
// layouts — chat, zen, and the hidden narrow_chat fallback — as a public,
// forkable LayoutProvider plugin wired by the Default Config. Core itself knows
// no concrete layouts; this package is the one that defines them, exactly like
// a user's own layout plugin (see the config.go template's SplitLayoutPlugin).
package layouts

import (
	"github.com/Ceinl/plums/capabilities"
	layout "github.com/Ceinl/plums/plums/layout"
)

// Plugin contributes the bundled chat/zen/narrow_chat layouts. `split` (and its
// narrow fallback) deliberately ship NOT here but as a user plugin — see the
// default config.go template — to prove a user-defined layout flows through the
// kernel registry exactly like a builtin.
type Plugin struct{}

func (*Plugin) Name() string                      { return "ui/layouts" }
func (*Plugin) Init(capabilities.Host, any) error { return nil }

func (*Plugin) Layouts() []capabilities.Layout {
	return []capabilities.Layout{
		layout.Named("chat", chatLayout()),
		layout.Named("zen", zenLayout()),
		layout.Hidden(layout.Named("narrow_chat", narrowChatLayout())),
	}
}

func chatLayout() capabilities.Node {
	return layout.Row(
		layout.Sessions().Width("15%").Height("100%"),
		layout.VerticalSeparator().Width(1).Height("100%"),
		layout.Column(
			layout.Chat().
				Width("100%").
				Height("grow").
				Padding(padding(0, 2, 0, 2)).
				Style(layout.ThemeBg(layout.ColorBgBase)),
			layout.InputBox().
				Width("100%").
				Height(9).
				Padding(padding(0, 2, 1, 2)).
				Style(layout.ThemeStyle(layout.ColorBgBase, layout.ColorText)),
		).
			Width("grow").
			Height("100%").
			AlignItems("center").
			Padding(padding(0, 0, 0, 0)).
			Style(layout.ThemeBg(layout.ColorBgBase)),
	).
		Width("100%").
		Height("100%").
		MinWidth("MinSplitLayoutWidth").
		Fallback("narrow_chat")
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

func narrowChatLayout() capabilities.Node {
	return layout.Column(
		layout.Component("sessions_horizontal").Width("100%").Height(3),
		layout.Chat().
			Width("100%").
			Height("grow").
			Padding(padding(0, 2, 0, 2)).
			Style(layout.ThemeBg(layout.ColorBgBase)),
		layout.InputBox().
			Width("100%").
			Height(9).
			Padding(padding(0, 2, 1, 2)).
			Style(layout.ThemeStyle(layout.ColorBgBase, layout.ColorText)),
	).
		Width("100%").
		Height("100%").
		AlignItems("center")
}

func padding(top, right, bottom, left float64) layout.Padding {
	return layout.Padding{
		Top:    &top,
		Right:  &right,
		Bottom: &bottom,
		Left:   &left,
	}
}
