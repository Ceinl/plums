package components

func (e *Editor) DeleteSelection() {
	if !e.HasSelection() {
		return
	}
	e.pushUndo()
	e.deleteSelectionNoUndo()
}

func (e *Editor) deleteSelectionNoUndo() {
	s, end := e.selBounds()
	if s.Row == end.Row {
		e.Content[s.Row] = append(e.Content[s.Row][:s.Col], e.Content[s.Row][end.Col:]...)
	} else {
		e.Content[s.Row] = append(e.Content[s.Row][:s.Col], e.Content[end.Row][end.Col:]...)
		e.Content = append(e.Content[:s.Row+1], e.Content[end.Row+1:]...)
	}
	e.Cursor.Pos = s
	e.ClearSelection()
	e.MakeDirty()
}

func (e *Editor) DeleteWordBackward() {
	e.RevealCursor()
	if e.HasSelection() {
		e.DeleteSelection()
		return
	}
	snapshot := e.snapshot()
	anchor := e.Cursor.Pos
	e.moveWordLeft()
	if e.Cursor.Pos.Equal(anchor) {
		return
	}
	e.pushUndoSnapshot(snapshot)
	e.Cursor.selActive = true
	e.Cursor.selAnchor = anchor
	e.deleteSelectionNoUndo()
}

func (e *Editor) DeleteWordForward() {
	e.RevealCursor()
	if e.HasSelection() {
		e.DeleteSelection()
		return
	}
	snapshot := e.snapshot()
	anchor := e.Cursor.Pos
	e.moveWordRight()
	if e.Cursor.Pos.Equal(anchor) {
		return
	}
	e.pushUndoSnapshot(snapshot)
	e.Cursor.selActive = true
	e.Cursor.selAnchor = anchor
	e.deleteSelectionNoUndo()
}

func (e *Editor) InsertRune(r rune) {
	e.RevealCursor()
	e.pushUndo()
	if e.HasSelection() {
		e.deleteSelectionNoUndo()
	}
	if r == '\n' {
		e.insertNewlineNoUndo()
		return
	}
	e.clamp()
	row := e.Cursor.Pos.Row
	col := e.Cursor.Pos.Col
	e.Content[row] = append(e.Content[row][:col], append([]rune{r}, e.Content[row][col:]...)...)
	e.Cursor.Pos.Col++
	e.MakeDirty()
}

func (e *Editor) InsertString(s string) {
	if s == "" {
		return
	}
	e.RevealCursor()
	e.pushUndo()
	if e.HasSelection() {
		e.deleteSelectionNoUndo()
	}
	for _, r := range s {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			e.insertNewlineNoUndo()
			continue
		}
		e.clamp()
		row := e.Cursor.Pos.Row
		col := e.Cursor.Pos.Col
		e.Content[row] = append(e.Content[row][:col], append([]rune{r}, e.Content[row][col:]...)...)
		e.Cursor.Pos.Col++
		e.MakeDirty()
	}
}

func (e *Editor) InsertNewline() {
	e.RevealCursor()
	e.pushUndo()
	if e.HasSelection() {
		e.deleteSelectionNoUndo()
	}
	e.insertNewlineNoUndo()
}

func (e *Editor) insertNewlineNoUndo() {
	e.clamp()
	row := e.Cursor.Pos.Row
	col := e.Cursor.Pos.Col
	right := make([]rune, len(e.Content[row])-col)
	copy(right, e.Content[row][col:])
	e.Content[row] = e.Content[row][:col]
	e.Content = append(e.Content[:row+1], append([][]rune{right}, e.Content[row+1:]...)...)
	e.Cursor.Pos.Row++
	e.Cursor.Pos.Col = 0
	e.MakeDirty()
}

func (e *Editor) DeleteBackward() {
	e.RevealCursor()
	if e.HasSelection() {
		e.DeleteSelection()
		return
	}
	e.clamp()
	row := e.Cursor.Pos.Row
	col := e.Cursor.Pos.Col
	if col > 0 {
		e.pushUndo()
		e.Content[row] = append(e.Content[row][:col-1], e.Content[row][col:]...)
		e.Cursor.Pos.Col--
	} else if row > 0 {
		e.pushUndo()
		prevLen := len(e.Content[row-1])
		e.Content[row-1] = append(e.Content[row-1], e.Content[row]...)
		e.Content = append(e.Content[:row], e.Content[row+1:]...)
		e.Cursor.Pos.Row--
		e.Cursor.Pos.Col = prevLen
	}
	e.MakeDirty()
}

func (e *Editor) DeleteForward() {
	e.RevealCursor()
	if e.HasSelection() {
		e.DeleteSelection()
		return
	}
	e.clamp()
	row := e.Cursor.Pos.Row
	col := e.Cursor.Pos.Col
	if col < len(e.Content[row]) {
		e.pushUndo()
		e.Content[row] = append(e.Content[row][:col], e.Content[row][col+1:]...)
	} else if row < len(e.Content)-1 {
		e.pushUndo()
		e.Content[row] = append(e.Content[row], e.Content[row+1]...)
		e.Content = append(e.Content[:row+1], e.Content[row+2:]...)
	}
	e.MakeDirty()
}

func (e *Editor) DeleteCurrentLine() {
	e.RevealCursor()
	e.clamp()
	row := e.Cursor.Pos.Row
	if len(e.Content) == 1 && len(e.Content[0]) == 0 {
		return
	}
	e.pushUndo()
	if len(e.Content) == 1 {
		e.Content[0] = []rune{}
	} else {
		e.Content = append(e.Content[:row], e.Content[row+1:]...)
	}
	if e.Cursor.Pos.Row >= len(e.Content) {
		e.Cursor.Pos.Row = len(e.Content) - 1
	}
	if e.Cursor.Pos.Col > len(e.Content[e.Cursor.Pos.Row]) {
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	}
	e.ClearSelection()
	e.MakeDirty()
}
