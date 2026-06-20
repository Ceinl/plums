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

	ed := state.Editor
	switch ev.Type {
	case keyboard.KeyPaste:
		state.ShowEditorDropdown()
		ed.InsertString(ev.Text)
		return true, false
	case keyboard.KeyMouseWheelUp:
		if ev.Mouse {
			return state.ScrollAt(ev.MouseX, ev.MouseY, 3), false
		}
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputVisible(3), false
		}
		return ed.Scroll(3), false
	case keyboard.KeyMouseWheelDown:
		if ev.Mouse {
			return state.ScrollAt(ev.MouseX, ev.MouseY, -3), false
		}
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputVisible(-3), false
		}
		return ed.Scroll(-3), false
	case keyboard.KeyMouseLeftDown:
		if state.PopupOpen {
			state.ClosePalette()
			return true, false
		}
		if state.SessionMouseDown(ev.MouseX, ev.MouseY) {
			return true, false
		}
		if state.Editor.MouseDown(ev.MouseX, ev.MouseY) {
			return true, false
		}
		if state.OutputMouseDown(ev.MouseX, ev.MouseY) {
			return true, false
		}
		return false, false
	case keyboard.KeyMouseLeftDrag:
		if !state.PopupOpen && state.Editor.MouseDrag(ev.MouseX, ev.MouseY) {
			return true, false
		}
		if !state.PopupOpen && state.OutputMouseDrag(ev.MouseX, ev.MouseY) {
			return true, false
		}
		return false, false
	case keyboard.KeyMouseLeftUp:
		if !state.PopupOpen && state.Editor.MouseUp(ev.MouseX, ev.MouseY) {
			return true, false
		}
		if !state.PopupOpen {
			text := state.OutputMouseUp(ev.MouseX, ev.MouseY)
			if text != "" {
				if err := writeClipboard(text, clipboardCmd); err != nil {
					state.AddMessage("system", "copy failed: "+err.Error())
				}
			}
			return true, false
		}
		return false, false
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
