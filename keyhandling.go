package main

import (
	"plums/internal/app"
	"plums/internal/keyboard"
)

func handleKey(state *app.State, ev keyboard.Event) (handled bool, quit bool) {
	ed := state.Editor
	if state.PopupOpen {
		switch ev.Type {
		case keyboard.KeyEscape:
			state.ClosePalette()
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
			switch ev.Ch {
			case 'h', 'H':
				if state.IsOutputPercentageSelected() {
					state.AdjustSelectedPaletteItem(1)
					return true, false
				}
				state.MovePalette(-1)
				return true, false
			case 'k', 'K':
				state.MovePalette(-1)
				return true, false
			case 'l', 'L':
				if state.IsOutputPercentageSelected() {
					state.AdjustSelectedPaletteItem(-1)
					return true, false
				}
				state.MovePalette(1)
				return true, false
			case 'j', 'J':
				state.MovePalette(1)
				return true, false
			}
		}
		return true, false
	}

	switch ev.Type {
	case keyboard.KeyCtrlC:
		return false, true
	case keyboard.KeyEnter:
		if !ev.Shift {
			ed.InsertNewline()
			return true, false
		}
		state.SubmitInput()
		return true, false
	case keyboard.KeyBackspace:
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordBackward()
		} else {
			ed.DeleteBackward()
		}
		return true, false
	case keyboard.KeyDelete:
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordForward()
		} else {
			ed.DeleteForward()
		}
		return true, false
	case keyboard.KeyTab:
		state.SwitchLayout()
		return true, false
	case keyboard.KeyEscape:
		if state.PopupOpen {
			state.TogglePopup()
			return true, false
		}
		ed.ClearSelection()
		return true, false
	case keyboard.KeyRune:
		if ev.Ctrl && (ev.Ch == 'P' || ev.Ch == 'p') {
			state.TogglePopup()
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
			case 'e', 'E':
				ed.MoveCursorEnd()
				return true, false
			case 'k', 'K':
				ed.DeleteCurrentLine()
				return true, false
			case 's', 'S':
				state.SubmitInput()
				return true, false
			case 't', 'T':
				state.CycleInfoView()
				return true, false
			}
			return false, false
		}
		if ev.Alt {
			// Don't insert Alt-modified characters into the editor.
			return false, false
		}
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
		if state.EffectiveLayout() == app.LayoutFullscreen {
			return ed.ScrollPage(1), false
		}
		return state.ScrollOutputPage(1), false
	case keyboard.KeyPageDown:
		if state.EffectiveLayout() == app.LayoutFullscreen {
			return ed.ScrollPage(-1), false
		}
		return state.ScrollOutputPage(-1), false
	case keyboard.KeyMouseWheelUp:
		if ev.Mouse {
			return state.ScrollAt(ev.MouseX, ev.MouseY, 3), false
		}
		if state.EffectiveLayout() == app.LayoutFullscreen {
			return ed.Scroll(3), false
		}
		return state.ScrollOutputVisible(3), false
	case keyboard.KeyMouseWheelDown:
		if ev.Mouse {
			return state.ScrollAt(ev.MouseX, ev.MouseY, -3), false
		}
		if state.EffectiveLayout() == app.LayoutFullscreen {
			return ed.Scroll(-3), false
		}
		return state.ScrollOutputVisible(-3), false
	case keyboard.KeyHome:
		if ev.Ctrl {
			if state.EffectiveLayout() == app.LayoutFullscreen {
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
			if state.EffectiveLayout() == app.LayoutFullscreen {
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
