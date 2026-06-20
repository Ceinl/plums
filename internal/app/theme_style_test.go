package app

import (
	"testing"

	"github.com/Ceinl/plums/internal/ui/tui/theme"
	publiclayout "github.com/Ceinl/plums/plums/layout"
)

func TestResolveStyleUsesActiveThemeToken(t *testing.T) {
	previous := theme.Snapshot()
	t.Cleanup(func() { theme.Apply(previous) })

	next := previous
	next.BgPanel = theme.Color{R: 3, G: 4, B: 5}
	next.Text = theme.Color{R: 6, G: 7, B: 8}
	theme.Apply(next)

	style := resolveStyle(StyleNode{
		BackgroundToken: publiclayout.ColorBgPanel,
		ForegroundToken: publiclayout.ColorText,
	})
	if got, want := style.GetBackground(), "\x1b[48;2;3;4;5m"; got != want {
		t.Fatalf("background = %q, want %q", got, want)
	}
	if got, want := style.GetForeground(), "\x1b[38;2;6;7;8m"; got != want {
		t.Fatalf("foreground = %q, want %q", got, want)
	}
}
