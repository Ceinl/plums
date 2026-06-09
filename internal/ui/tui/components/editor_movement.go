package components

import "unicode"

// ── Plain cursor movement ──────────────────────────────────────────────────

func (e *Editor) MoveCursorLeft() {
	e.RevealCursor()
	if e.HasSelection() {
		s, _ := e.selBounds()
		e.Cursor.Pos = s
		e.ClearSelection()
		return
	}
	if e.Cursor.Pos.Col > 0 {
		e.Cursor.Pos.Col--
	} else if e.Cursor.Pos.Row > 0 {
		e.Cursor.Pos.Row--
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	}
	e.Cursor.lastCol = uint(e.Cursor.Pos.Col)
}

func (e *Editor) MoveCursorRight() {
	e.RevealCursor()
	if e.HasSelection() {
		_, end := e.selBounds()
		e.Cursor.Pos = end
		e.ClearSelection()
		return
	}
	if e.Cursor.Pos.Col < len(e.Content[e.Cursor.Pos.Row]) {
		e.Cursor.Pos.Col++
	} else if e.Cursor.Pos.Row < len(e.Content)-1 {
		e.Cursor.Pos.Row++
		e.Cursor.Pos.Col = 0
	}
	e.Cursor.lastCol = uint(e.Cursor.Pos.Col)
}

func (e *Editor) MoveCursorUp() {
	e.RevealCursor()
	if e.HasSelection() {
		s, _ := e.selBounds()
		e.Cursor.Pos = s
		e.ClearSelection()
		return
	}
	if e.Cursor.Pos.Row == 0 {
		e.Cursor.Pos.Col = 0
	} else {
		e.Cursor.Pos.Row--
		e.Cursor.Pos.Col = int(e.LastCursorUpdate(e.Content[e.Cursor.Pos.Row]))
	}
}

func (e *Editor) MoveCursorDown() {
	e.RevealCursor()
	if e.HasSelection() {
		_, end := e.selBounds()
		e.Cursor.Pos = end
		e.ClearSelection()
		return
	}
	if e.Cursor.Pos.Row < len(e.Content)-1 {
		e.Cursor.Pos.Row++
		e.Cursor.Pos.Col = int(e.LastCursorUpdate(e.Content[e.Cursor.Pos.Row]))
	} else {
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	}
}

func (e *Editor) MoveCursorHome() {
	e.RevealCursor()
	if e.HasSelection() {
		s, _ := e.selBounds()
		e.Cursor.Pos = s
		e.ClearSelection()
		return
	}
	e.Cursor.Pos.Col = 0
	e.Cursor.lastCol = 0
}

func (e *Editor) MoveCursorEnd() {
	e.RevealCursor()
	if e.HasSelection() {
		_, end := e.selBounds()
		e.Cursor.Pos = end
		e.ClearSelection()
		return
	}
	e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	e.Cursor.lastCol = uint(e.Cursor.Pos.Col)
}

// ── Word movement ──────────────────────────────────────────────────────────

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func (e *Editor) moveWordLeft() {
	if e.Cursor.Pos.Col == 0 {
		if e.Cursor.Pos.Row == 0 {
			return
		}
		e.Cursor.Pos.Row--
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
		if e.Cursor.Pos.Col == 0 {
			return
		}
	}
	row := e.Cursor.Pos.Row
	col := e.Cursor.Pos.Col
	line := e.Content[row]
	for col > 0 && unicode.IsSpace(line[col-1]) {
		col--
	}
	if col > 0 {
		if isWordRune(line[col-1]) {
			for col > 0 && isWordRune(line[col-1]) {
				col--
			}
		} else {
			for col > 0 && !isWordRune(line[col-1]) && !unicode.IsSpace(line[col-1]) {
				col--
			}
		}
	}
	e.Cursor.Pos.Col = col
}

func (e *Editor) moveWordRight() {
	row := e.Cursor.Pos.Row
	line := e.Content[row]
	col := e.Cursor.Pos.Col
	if col == len(line) {
		if row >= len(e.Content)-1 {
			return
		}
		e.Cursor.Pos.Row++
		e.Cursor.Pos.Col = 0
		row = e.Cursor.Pos.Row
		line = e.Content[row]
		col = 0
		if len(line) == 0 {
			return
		}
	}
	for col < len(line) && unicode.IsSpace(line[col]) {
		col++
	}
	if col < len(line) {
		if isWordRune(line[col]) {
			for col < len(line) && isWordRune(line[col]) {
				col++
			}
		} else {
			for col < len(line) && !isWordRune(line[col]) && !unicode.IsSpace(line[col]) {
				col++
			}
		}
	}
	e.Cursor.Pos.Col = col
}

func (e *Editor) MoveWordLeft() {
	e.RevealCursor()
	if e.HasSelection() {
		s, _ := e.selBounds()
		e.Cursor.Pos = s
		e.ClearSelection()
		return
	}
	e.moveWordLeft()
	e.Cursor.lastCol = uint(e.Cursor.Pos.Col)
}

func (e *Editor) MoveWordRight() {
	e.RevealCursor()
	if e.HasSelection() {
		_, end := e.selBounds()
		e.Cursor.Pos = end
		e.ClearSelection()
		return
	}
	e.moveWordRight()
	e.Cursor.lastCol = uint(e.Cursor.Pos.Col)
}
