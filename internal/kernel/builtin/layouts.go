package builtin

import (
	"github.com/Ceinl/plums/capabilities"
	publiclayout "github.com/Ceinl/plums/plums/layout"
)

// DefaultLayouts returns the bundled layouts. Note: `split` (and its narrow
// fallback) deliberately ship NOT here but as a user plugin — see the default
// config.go template — to prove a user-defined layout flows through the kernel
// registry exactly like a builtin.
func DefaultLayouts() []capabilities.Layout {
	return []capabilities.Layout{
		publiclayout.Named("chat", chatLayout()),
		publiclayout.Named("zen", zenLayout()),
		publiclayout.Hidden(publiclayout.Named("narrow_chat", narrowChatLayout())),
		publiclayout.Named("fullscreen", publiclayout.Component("fullscreen_view")),
	}
}

func chatLayout() capabilities.Node {
	return publiclayout.Row(
		publiclayout.Sessions().Width("15%").Height("100%"),
		publiclayout.VerticalSeparator().Width(1).Height("100%"),
		publiclayout.Column(
			publiclayout.Chat().
				Width("100%").
				Height("grow").
				Padding(padding(0, 2, 0, 2)).
				Style(styleBg(22, 20, 27)),
			publiclayout.InputBox().
				Width("100%").
				Height(9).
				Padding(padding(0, 2, 1, 2)).
				Style(styleBgFg(22, 20, 27, 232, 229, 241)),
		).
			Width("grow").
			Height("100%").
			AlignItems("center").
			Padding(padding(0, 0, 0, 0)).
			Style(styleBg(22, 20, 27)),
	).
		Width("100%").
		Height("100%").
		MinWidth("MinSplitLayoutWidth").
		Fallback("narrow_chat")
}

func zenLayout() capabilities.Node {
	return publiclayout.Column(
		publiclayout.Chat().
			Width("100%").
			Height("grow").
			Padding(padding(2, 6, 1, 6)).
			Style(styleBg(24, 24, 27)),
		publiclayout.InputBox().
			Width("100%").
			Height(7).
			Padding(padding(0, 6, 2, 6)).
			Style(styleBgFg(24, 24, 27, 205, 205, 212)),
	).
		Width("100%").
		Height("100%").
		AlignItems("center").
		Style(styleBg(24, 24, 27))
}

func narrowChatLayout() capabilities.Node {
	return publiclayout.Column(
		publiclayout.Component("sessions_horizontal").Width("100%").Height(3),
		publiclayout.Chat().
			Width("100%").
			Height("grow").
			Padding(padding(0, 2, 0, 2)).
			Style(styleBg(22, 20, 27)),
		publiclayout.InputBox().
			Width("100%").
			Height(9).
			Padding(padding(0, 2, 1, 2)).
			Style(styleBgFg(22, 20, 27, 232, 229, 241)),
	).
		Width("100%").
		Height("100%").
		AlignItems("center")
}

func padding(top, right, bottom, left float64) publiclayout.Padding {
	return publiclayout.Padding{
		Top:    &top,
		Right:  &right,
		Bottom: &bottom,
		Left:   &left,
	}
}

func styleBg(r, g, b uint8) publiclayout.Style {
	return publiclayout.Style{Background: []uint8{r, g, b}}
}

func styleBgFg(br, bg, bb, fr, fg, fb uint8) publiclayout.Style {
	return publiclayout.Style{
		Background: []uint8{br, bg, bb},
		Foreground: []uint8{fr, fg, fb},
	}
}
