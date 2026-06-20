package builtincfg

import "testing"

func TestOptsIncludesDefaultKeybinds(t *testing.T) {
	got := Opts().Keybinds
	want := DefaultKeybinds()
	if len(got) != len(want) {
		t.Fatalf("keybinds = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keybinds = %+v, want %+v", got, want)
		}
	}
}
