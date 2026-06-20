package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

func TestPaletteForStateUsesExplicitTheme(t *testing.T) {
	state := NewState(80, 24)
	state.Layout = LayoutChat
	state.SetTheme(capabilities.Theme{Name: "zen"})

	if got := paletteForState(state); got.BgBase != theme.Zen.BgBase {
		t.Fatalf("palette bg = %+v, want zen %+v", got.BgBase, theme.Zen.BgBase)
	}
}

func TestPaletteForStateAllowsDefaultThemeOverride(t *testing.T) {
	state := NewState(80, 24)
	state.Layout = LayoutZen
	state.SetTheme(capabilities.Theme{Name: "default"})

	if got := paletteForState(state); got.BgBase != theme.Default.BgBase {
		t.Fatalf("palette bg = %+v, want default %+v", got.BgBase, theme.Default.BgBase)
	}
}

func TestPaletteForStateFallsBackToLayoutTheme(t *testing.T) {
	state := NewState(80, 24)
	state.Layout = LayoutZen

	if got := paletteForState(state); got.BgBase != theme.Zen.BgBase {
		t.Fatalf("palette bg = %+v, want layout zen %+v", got.BgBase, theme.Zen.BgBase)
	}
}
