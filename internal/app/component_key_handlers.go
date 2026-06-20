package app

import "github.com/Ceinl/plums/capabilities"

func handlePaletteKey(state *State, ev capabilities.KeyEvent) bool {
	if state == nil || !state.PopupOpen {
		return false
	}
	switch ev.Key {
	case "escape":
		state.ClosePalette()
		return true
	case "backspace":
		state.DeletePaletteRune()
		return true
	case "delete":
		state.ClearPaletteSearch()
		return true
	case "up":
		state.MovePalette(-1)
		return true
	case "down":
		state.MovePalette(1)
		return true
	case "left":
		if state.IsOutputPercentageSelected() {
			state.AdjustSelectedPaletteItem(1)
			return true
		}
		state.MovePalette(-1)
		return true
	case "right":
		if state.IsOutputPercentageSelected() {
			state.AdjustSelectedPaletteItem(-1)
			return true
		}
		state.MovePalette(1)
		return true
	case "enter":
		state.SelectPaletteItem()
		return true
	}
	if ev.Ctrl && (ev.Rune == 'N' || ev.Rune == 'n') {
		state.MovePalette(1)
		return true
	}
	if ev.Ctrl && (ev.Rune == 'K' || ev.Rune == 'k') {
		state.ClearPaletteSearch()
		return true
	}
	if ev.Ctrl && (ev.Rune == 'C' || ev.Rune == 'c') {
		return false
	}
	if ev.Ctrl || ev.Alt || ev.Cmd {
		return true
	}
	if ev.Rune != 0 {
		state.InsertPaletteRune(ev.Rune)
		return true
	}
	return false
}

func handleEditorKey(state *State, ctx capabilities.Ctx, ev capabilities.KeyEvent) bool {
	if state == nil || state.Editor == nil || state.PopupOpen {
		return false
	}
	if state.EditorDropdownOpen() {
		switch ev.Key {
		case "escape":
			state.CloseEditorDropdown()
			return true
		case "up", "left":
			state.MoveEditorDropdown(-1)
			return true
		case "down", "right":
			state.MoveEditorDropdown(1)
			return true
		case "enter":
			if state.SubmitExactSlashCommand() {
				return true
			}
			state.SelectEditorDropdownItem()
			return true
		}
	}

	ed := state.Editor
	switch ev.Key {
	case "enter":
		if state.EffectiveLayout() == LayoutSplit {
			if ev.Shift {
				state.SubmitInput()
				return true
			}
			ed.InsertNewline()
			return true
		}
		if ev.Shift {
			ed.InsertNewline()
			return true
		}
		state.SubmitInput()
		return true
	case "backspace":
		state.ShowEditorDropdown()
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordBackward()
		} else {
			ed.DeleteBackward()
		}
		return true
	case "delete":
		state.ShowEditorDropdown()
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordForward()
		} else {
			ed.DeleteForward()
		}
		return true
	case "tab":
		if state.EffectiveLayout() == LayoutSplit {
			state.CycleInfoView()
			return true
		}
		ed.InsertRune('\t')
		return true
	case "escape":
		ed.ClearSelection()
		return true
	case "left":
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
		return true
	case "right":
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
		return true
	case "up":
		if ev.Alt && !ev.Shift && !ev.Ctrl {
			return state.ScrollOutputVisible(1)
		}
		if ev.Shift {
			ed.SelectUp()
		} else {
			ed.MoveCursorUp()
		}
		return true
	case "down":
		if ev.Alt && !ev.Shift && !ev.Ctrl {
			return state.ScrollOutputVisible(-1)
		}
		if ev.Shift {
			ed.SelectDown()
		} else {
			ed.MoveCursorDown()
		}
		return true
	case "pageup":
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputPage(1)
		}
		return ed.ScrollPage(1)
	case "pagedown":
		if state.LayoutScrollsOutput() {
			return state.ScrollOutputPage(-1)
		}
		return ed.ScrollPage(-1)
	case "home":
		if ev.Ctrl {
			return ed.ScrollTop()
		}
		if ev.Shift {
			ed.SelectHome()
		} else {
			ed.MoveCursorHome()
		}
		return true
	case "end":
		if ev.Ctrl {
			return ed.ScrollBottom()
		}
		if ev.Shift {
			ed.SelectEnd()
		} else {
			ed.MoveCursorEnd()
		}
		return true
	}

	if ev.Cmd {
		switch ev.Rune {
		case 'c', 'C':
			if ed.HasSelection() && ctx != nil {
				ctx.Copy(ed.SelectedText())
			}
			return true
		case 'z', 'Z':
			ed.Undo()
			return true
		}
		return true
	}
	if ev.Alt && !ev.Ctrl {
		switch ev.Rune {
		case 'b', 'B':
			ed.MoveWordLeft()
			return true
		case 'f', 'F':
			ed.MoveWordRight()
			return true
		case 'd', 'D':
			ed.DeleteWordForward()
			return true
		}
	}
	if ev.Ctrl {
		switch ev.Rune {
		case 'a', 'A':
			ed.MoveCursorHome()
			return true
		case 'e', 'E':
			ed.MoveCursorEnd()
			return true
		case 'k', 'K':
			ed.DeleteCurrentLine()
			return true
		case 'z', 'Z':
			return ed.Undo()
		}
		return false
	}
	if ev.Alt {
		return false
	}
	if ev.Rune != 0 {
		state.ShowEditorDropdown()
		ed.InsertRune(ev.Rune)
		return true
	}
	return false
}
