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
	if state.PopupOpen {
		switch ev.Type {
		case keyboard.KeyEscape:
			state.ClosePalette()
			return true, false
		case keyboard.KeyBackspace:
			state.DeletePaletteRune()
			return true, false
		case keyboard.KeyDelete:
			state.ClearPaletteSearch()
			return true, false
		case keyboard.KeyArrowUp:
			state.MovePalette(-1)
			return true, false
		case keyboard.KeyArrowDown:
			state.MovePalette(1)
			return true, false
		case keyboard.KeyArrowLeft:
			if state.IsOutputPercentageSelected() {
				state.AdjustSelectedPaletteItem(1)
				return true, false
			}
			state.MovePalette(-1)
			return true, false
		case keyboard.KeyArrowRight:
			if state.IsOutputPercentageSelected() {
				state.AdjustSelectedPaletteItem(-1)
				return true, false
			}
			state.MovePalette(1)
			return true, false
		case keyboard.KeyEnter:
			state.SelectPaletteItem()
			return true, false
		case keyboard.KeyRune:
			if ev.Ctrl && (ev.Ch == 'P' || ev.Ch == 'p') {
				state.ClosePalette()
				return true, false
			}
			if ev.Ctrl && (ev.Ch == 'N' || ev.Ch == 'n') {
				state.MovePalette(1)
				return true, false
			}
			if ev.Ctrl && (ev.Ch == 'K' || ev.Ch == 'k') {
				state.ClearPaletteSearch()
				return true, false
			}
			if ev.Ctrl || ev.Alt {
				return true, false
			}
			state.InsertPaletteRune(ev.Ch)
			return true, false
		}
		return true, false
	}

	if state.EditorDropdownOpen() {
		switch ev.Type {
		case keyboard.KeyEscape:
			state.CloseEditorDropdown()
			return true, false
		case keyboard.KeyArrowUp, keyboard.KeyArrowLeft:
			state.MoveEditorDropdown(-1)
			return true, false
		case keyboard.KeyArrowDown, keyboard.KeyArrowRight:
			state.MoveEditorDropdown(1)
			return true, false
		case keyboard.KeyEnter:
			if state.SubmitExactSlashCommand() {
				return true, false
			}
			state.SelectEditorDropdownItem()
			return true, false
		}
	}

	switch ev.Type {
	case keyboard.KeyEnter:
		if state.EffectiveLayout() == LayoutSplit {
			if ev.Shift {
				state.SubmitInput()
				return true, false
			}
			ed.InsertNewline()
			return true, false
		}
		// Chat and fullscreen layouts: Enter sends, Shift+Enter adds newline.
		if ev.Shift {
			ed.InsertNewline()
			return true, false
		}
		state.SubmitInput()
		return true, false
	case keyboard.KeyBackspace:
		state.ShowEditorDropdown()
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordBackward()
		} else {
			ed.DeleteBackward()
		}
		return true, false
	case keyboard.KeyDelete:
		state.ShowEditorDropdown()
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordForward()
		} else {
			ed.DeleteForward()
		}
		return true, false
	case keyboard.KeyTab:
		if state.EffectiveLayout() == LayoutFullscreen {
			if ev.Shift {
				state.CycleFullscreenTab(-1)
			} else {
				state.CycleFullscreenTab(1)
			}
			return true, false
		}
		if state.EffectiveLayout() == LayoutSplit {
			state.CycleInfoView()
			return true, false
		}
		// Chat layout: insert a tab character in the editor.
		ed.InsertRune('\t')
		return true, false
	case keyboard.KeyPaste:
		state.ShowEditorDropdown()
		ed.InsertString(ev.Text)
		return true, false
	case keyboard.KeyEscape:
		ed.ClearSelection()
		return true, false
	case keyboard.KeyRune:
		if ev.Ctrl && (ev.Ch == 'P' || ev.Ch == 'p') {
			state.TogglePopup()
			return true, false
		}
		if ev.Cmd {
			switch ev.Ch {
			case 'c', 'C':
				copyEditorSelection(state, clipboardCmd)
				return true, false
			case 'z', 'Z':
				ed.Undo()
				return true, false
			}
			return true, false
		}
		// Alt+b / Alt+f: readline-style word jump (emitted by Terminal.app and
		// many other macOS terminals when the user presses Option+Left/Right).
		if ev.Alt && !ev.Ctrl {
			switch ev.Ch {
			case 'b', 'B': // Option+Left in Terminal.app
				ed.MoveWordLeft()
				return true, false
			case 'f', 'F': // Option+Right in Terminal.app
				ed.MoveWordRight()
				return true, false
			case 'd', 'D': // Alt+d = delete word forward (readline)
				ed.DeleteWordForward()
				return true, false
			}
		}
		if ev.Ctrl {
			switch ev.Ch {
			case 'a', 'A':
				ed.MoveCursorHome()
				return true, false
			case 'c', 'C':
				if copyEditorSelection(state, clipboardCmd) {
					return true, false
				}
				return false, true
			case 'e', 'E':
				ed.MoveCursorEnd()
				return true, false
			case 'k', 'K':
				ed.DeleteCurrentLine()
				return true, false
			case 's', 'S':
				state.SubmitInput()
				return true, false
			case 'z', 'Z':
				return ed.Undo(), false
			}
			return false, false
		}
		if ev.Alt {
			// Don't insert Alt-modified characters into the editor.
			return false, false
		}
		state.ShowEditorDropdown()
		ed.InsertRune(ev.Ch)
		return true, false
	case keyboard.KeyArrowLeft:
		switch {
		case (ev.Ctrl || ev.Alt) && ev.Shift:
			ed.SelectWordLeft()
		case ev.Ctrl || ev.Alt:
			ed.MoveWordLeft()
		case ev.Shift:
			ed.SelectLeft()
		default:
			ed.MoveCursorLeft()
		}
		return true, false
	case keyboard.KeyArrowRight:
		switch {
		case (ev.Ctrl || ev.Alt) && ev.Shift:
			ed.SelectWordRight()
		case ev.Ctrl || ev.Alt:
			ed.MoveWordRight()
		case ev.Shift:
			ed.SelectRight()
		default:
			ed.MoveCursorRight()
		}
		return true, false
	case keyboard.KeyArrowUp:
		if ev.Alt && !ev.Shift && !ev.Ctrl {
			return state.ScrollOutputVisible(1), false
		}
		if ev.Shift {
			ed.SelectUp()
		} else {
			ed.MoveCursorUp()
		}
		return true, false
	case keyboard.KeyArrowDown:
		if ev.Alt && !ev.Shift && !ev.Ctrl {
			return state.ScrollOutputVisible(-1), false
		}
		if ev.Shift {
			ed.SelectDown()
		} else {
			ed.MoveCursorDown()
		}
		return true, false
	case keyboard.KeyPageUp:
		if state.EffectiveLayout() == LayoutFullscreen && !state.FullscreenShowsEditor() {
			return state.ScrollOutputPage(1), false
		}
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputPage(1), false
		}
		return ed.ScrollPage(1), false
	case keyboard.KeyPageDown:
		if state.EffectiveLayout() == LayoutFullscreen && !state.FullscreenShowsEditor() {
			return state.ScrollOutputPage(-1), false
		}
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputPage(-1), false
		}
		return ed.ScrollPage(-1), false
	case keyboard.KeyMouseWheelUp:
		if ev.Mouse {
			return state.ScrollAt(ev.MouseX, ev.MouseY, 3), false
		}
		if state.EffectiveLayout() == LayoutFullscreen && !state.FullscreenShowsEditor() {
			return state.ScrollOutputVisible(3), false
		}
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputVisible(3), false
		}
		return ed.Scroll(3), false
	case keyboard.KeyMouseWheelDown:
		if ev.Mouse {
			return state.ScrollAt(ev.MouseX, ev.MouseY, -3), false
		}
		if state.EffectiveLayout() == LayoutFullscreen && !state.FullscreenShowsEditor() {
			return state.ScrollOutputVisible(-3), false
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
	case keyboard.KeyHome:
		if ev.Ctrl {
			if state.FullscreenShowsEditor() {
				return ed.ScrollTop(), false
			}
			return state.ScrollOutputPage(1 << 20), false
		}
		if ev.Shift {
			ed.SelectHome()
		} else {
			ed.MoveCursorHome()
		}
		return true, false
	case keyboard.KeyEnd:
		if ev.Ctrl {
			if state.FullscreenShowsEditor() {
				return ed.ScrollBottom(), false
			}
			return state.ScrollOutputBottom(), false
		}
		if ev.Shift {
			ed.SelectEnd()
		} else {
			ed.MoveCursorEnd()
		}
		return true, false
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
