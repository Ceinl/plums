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

func TestOptsIncludesThemeAndClipboardDefaults(t *testing.T) {
	opts := Opts()
	if opts.Theme.Name != "default" {
		t.Fatalf("theme = %+v, want default", opts.Theme)
	}
	if opts.ClipboardCommand != "pbcopy" {
		t.Fatalf("clipboard command = %q, want pbcopy", opts.ClipboardCommand)
	}
}
