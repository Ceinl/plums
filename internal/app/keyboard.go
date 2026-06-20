package app

import (
	"github.com/Ceinl/plums/internal/keyboard"
)

// HandleKey processes a single keyboard event and updates state. It returns
// handled=true when the event was consumed, and quit=true when the application
// should exit.
func HandleKey(state *State, ev keyboard.Event, clipboardCmd string) (handled bool, quit bool) {
	ev = normalizeKeyEvent(ev)
	if ev.Type == keyboard.KeyCtrlC {
		if copyEditorSelection(state, clipboardCmd) {
			return true, false
		}
		if ev.Cmd {
			return true, false
		}
		return false, true
	}

	return false, false
}

func normalizeKeyEvent(ev keyboard.Event) keyboard.Event {
	if ev.Type == keyboard.KeyRune && ev.Ch == '\t' {
		ev.Type = keyboard.KeyTab
		ev.Ch = 0
	}
	if ev.Type == keyboard.KeyRune && ev.Ctrl && (ev.Ch == 'I' || ev.Ch == 'i') {
		ev.Type = keyboard.KeyTab
		ev.Ch = 0
		ev.Ctrl = false
	}
	return ev
}
