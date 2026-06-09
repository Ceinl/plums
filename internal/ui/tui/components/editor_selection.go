package components

import "strings"

func (e *Editor) selBounds() (CursorPos, CursorPos) {
	if e.Cursor.Pos.Less(e.Cursor.selAnchor) {
		return e.Cursor.Pos, e.Cursor.selAnchor
	}
	return e.Cursor.selAnchor, e.Cursor.Pos
}

func (e *Editor) HasSelection() bool {
	return e.Cursor.selActive && !e.Cursor.Pos.Equal(e.Cursor.selAnchor)
}

func (e *Editor) ClearSelection() {
	e.Cursor.selActive = false
	e.mouseSelecting = false
}

func (e *Editor) IsPoint(x, y int) bool {
	return x >= e.x && x < e.x+e.w && y >= e.y && y < e.y+e.h
}

func (e *Editor) cursorPosForScreenPoint(x, y int) CursorPos {
	e.clamp()
	visLines := e.computeVisualLines()
	if len(visLines) == 0 {
		return CursorPos{}
	}

	row := y - e.y
	if row < 0 {
		row = 0
	}
	if row >= e.h {
		row = e.h - 1
	}
	vlIdx := e.scrollY + row
	if vlIdx < 0 {
		vlIdx = 0
	}
	if vlIdx >= len(visLines) {
		lastRow := len(e.Content) - 1
		return CursorPos{Row: lastRow, Col: len(e.Content[lastRow])}
	}

	vl := visLines[vlIdx]
	contentX := e.x + 4
	if e.inputBoxMode {
		contentX = e.x + 1
	}
	col := vl.start + x - contentX
	if col < vl.start {
		col = vl.start
	}
	if col > vl.end {
		col = vl.end
	}
	return CursorPos{Row: vl.row, Col: col}
}

func (e *Editor) MouseDown(x, y int) bool {
	if !e.IsPoint(x, y) {
		return false
	}
	pos := e.cursorPosForScreenPoint(x, y)
	e.Cursor.Pos = pos
	e.Cursor.selAnchor = pos
	e.Cursor.selActive = true
	e.mouseSelecting = true
	e.manualScroll = true
	e.MakeDirty()
	return true
}

func (e *Editor) MouseDrag(x, y int) bool {
	if !e.mouseSelecting {
		return false
	}
	e.Cursor.Pos = e.cursorPosForScreenPoint(x, y)
	e.Cursor.selActive = true
	e.MakeDirty()
	return true
}

func (e *Editor) MouseUp(x, y int) bool {
	if !e.mouseSelecting {
		return false
	}
	e.Cursor.Pos = e.cursorPosForScreenPoint(x, y)
	e.mouseSelecting = false
	if !e.HasSelection() {
		e.Cursor.selActive = false
	}
	e.MakeDirty()
	return true
}

func (e *Editor) beginSelectionIfNeeded() {
	if !e.Cursor.selActive {
		e.Cursor.selActive = true
		e.Cursor.selAnchor = e.Cursor.Pos
	}
}

func (e *Editor) isSelected(row, col int) bool {
	if !e.HasSelection() {
		return false
	}
	s, end := e.selBounds()
	pos := CursorPos{Row: row, Col: col}
	return !pos.Less(s) && pos.Less(end)
}

func (e *Editor) SelectedText() string {
	if !e.HasSelection() {
		return ""
	}
	s, end := e.selBounds()
	if s.Row == end.Row {
		return string(e.Content[s.Row][s.Col:end.Col])
	}
	var b strings.Builder
	b.WriteString(string(e.Content[s.Row][s.Col:]))
	for row := s.Row + 1; row < end.Row; row++ {
		b.WriteRune('\n')
		b.WriteString(string(e.Content[row]))
	}
	b.WriteRune('\n')
	b.WriteString(string(e.Content[end.Row][:end.Col]))
	return b.String()
}

func (e *Editor) SelectAll() {
	if len(e.Content) == 0 || (len(e.Content) == 1 && len(e.Content[0]) == 0) {
		e.ClearSelection()
		return
	}
	lastRow := len(e.Content) - 1
	lastCol := len(e.Content[lastRow])
	e.Cursor.selActive = true
	e.Cursor.selAnchor = CursorPos{Row: 0, Col: 0}
	e.Cursor.Pos = CursorPos{Row: lastRow, Col: lastCol}
}

// ── Selection movement ─────────────────────────────────────────────────────

func (e *Editor) SelectLeft() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	if e.Cursor.Pos.Col > 0 {
		e.Cursor.Pos.Col--
	} else if e.Cursor.Pos.Row > 0 {
		e.Cursor.Pos.Row--
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	}
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectRight() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	if e.Cursor.Pos.Col < len(e.Content[e.Cursor.Pos.Row]) {
		e.Cursor.Pos.Col++
	} else if e.Cursor.Pos.Row < len(e.Content)-1 {
		e.Cursor.Pos.Row++
		e.Cursor.Pos.Col = 0
	}
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectUp() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	if e.Cursor.Pos.Row == 0 {
		e.Cursor.Pos.Col = 0
	} else {
		e.Cursor.Pos.Row--
		e.Cursor.Pos.Col = int(e.LastCursorUpdate(e.Content[e.Cursor.Pos.Row]))
	}
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectDown() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	if e.Cursor.Pos.Row < len(e.Content)-1 {
		e.Cursor.Pos.Row++
		e.Cursor.Pos.Col = int(e.LastCursorUpdate(e.Content[e.Cursor.Pos.Row]))
	} else {
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	}
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectHome() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	e.Cursor.Pos.Col = 0
	e.Cursor.lastCol = 0
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectEnd() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	e.Cursor.lastCol = uint(e.Cursor.Pos.Col)
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectWordLeft() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	e.moveWordLeft()
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}

func (e *Editor) SelectWordRight() {
	e.RevealCursor()
	e.beginSelectionIfNeeded()
	e.moveWordRight()
	if e.Cursor.Pos.Equal(e.Cursor.selAnchor) {
		e.ClearSelection()
	}
}
