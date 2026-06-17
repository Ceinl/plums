package components

import (
	"testing"

	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// TestRefreshColorsTracksPaletteSwap verifies the cached component colours
// follow theme.Apply once RefreshColors runs, and restore on the default
// palette — the mechanism that lets the zen layout recolour shared chrome.
func TestRefreshColorsTracksPaletteSwap(t *testing.T) {
	t.Cleanup(func() { theme.Apply(theme.Default); RefreshColors() })

	theme.Apply(theme.Default)
	RefreshColors()
	def := infoTabsBg

	theme.Apply(theme.Zen)
	RefreshColors()
	if infoTabsBg == def {
		t.Fatal("expected cached infoTabsBg to change under the zen palette")
	}
	if infoTabsBg != theme.BgBase.Bg() {
		t.Fatalf("cached colour out of sync with active palette: %q vs %q", infoTabsBg, theme.BgBase.Bg())
	}

	theme.Apply(theme.Default)
	RefreshColors()
	if infoTabsBg != def {
		t.Fatal("expected cached infoTabsBg to restore under the default palette")
	}
}
