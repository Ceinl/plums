package components

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Ceinl/plums/internal/layout"
	"github.com/Ceinl/plums/internal/screen"
)

type CursorPos struct {
	Row int
	Col int
}

func (a CursorPos) Less(b CursorPos) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}

func (a CursorPos) Equal(b CursorPos) bool {
	return a.Row == b.Row && a.Col == b.Col
}

type Cursor struct {
	Pos       CursorPos
	selAnchor CursorPos
	selActive bool
	lastCol   uint
}

type editorVisualLine struct {
	row   int
	start int
	end   int
}

type Editor struct {
	isDirty bool

	Content [][]rune
	Cursor  Cursor
	undo    []editorSnapshot

	style  layout.Style
	parent layout.Component

	x, y int
	w, h int

	scrollY      int
	manualScroll bool
	lineCount    int

	cursorScreenX  int
	cursorScreenY  int
	mouseSelecting bool
}

type editorSnapshot struct {
	content      [][]rune
	cursor       Cursor
	scrollY      int
	manualScroll bool
}

const maxUndoSnapshots = 100

func NewTextEditor() *Editor {
	return &Editor{
		Content: [][]rune{{}},
	}
}

func (e *Editor) IsDirty() bool                { return e.isDirty }
func (e *Editor) MakeDirty()                   { e.isDirty = true }
func (e *Editor) ClearDirty()                  { e.isDirty = false }
func (e *Editor) GetStyle() layout.Style       { return e.style }
func (e *Editor) SetParent(p layout.Component) { e.parent = p }
func (e *Editor) SetStyle(s layout.Style)      { e.style = s }

func (e *Editor) Layout(x, y, w, h int) {
	e.x, e.y, e.w, e.h = x, y, w, h
}

func (e *Editor) SetMultiline(v bool) {}
func (e *Editor) IsMultiline() bool   { return true }

func (e *Editor) clamp() {
	if len(e.Content) == 0 {
		e.Content = [][]rune{{}}
	}
	if e.Cursor.Pos.Row < 0 {
		e.Cursor.Pos.Row = 0
	}
	if e.Cursor.Pos.Row >= len(e.Content) {
		e.Cursor.Pos.Row = len(e.Content) - 1
	}
	if e.Cursor.Pos.Col < 0 {
		e.Cursor.Pos.Col = 0
	}
	if e.Cursor.Pos.Col > len(e.Content[e.Cursor.Pos.Row]) {
		e.Cursor.Pos.Col = len(e.Content[e.Cursor.Pos.Row])
	}
}

func (e *Editor) SetContent(s string) {
	e.undo = nil
	if s == "" {
		e.Content = [][]rune{{}}
		e.Cursor.Pos = CursorPos{}
		e.scrollY = 0
		e.manualScroll = false
		e.ClearSelection()
		e.isDirty = true
		return
	}
	rawLines := strings.Split(s, "\n")
	e.Content = make([][]rune, len(rawLines))
	for i, line := range rawLines {
		e.Content[i] = []rune(line)
	}
	lastRow := len(e.Content) - 1
	e.Cursor.Pos = CursorPos{Row: lastRow, Col: len(e.Content[lastRow])}
	e.scrollY = 0
	e.manualScroll = false
	e.ClearSelection()
	e.isDirty = true
}

func cloneContent(content [][]rune) [][]rune {
	clone := make([][]rune, len(content))
	for i, row := range content {
		clone[i] = append([]rune(nil), row...)
	}
	return clone
}

func (e *Editor) snapshot() editorSnapshot {
	return editorSnapshot{
		content:      cloneContent(e.Content),
		cursor:       e.Cursor,
		scrollY:      e.scrollY,
		manualScroll: e.manualScroll,
	}
}

func (e *Editor) pushUndoSnapshot(snapshot editorSnapshot) {
	if len(e.undo) >= maxUndoSnapshots {
		copy(e.undo, e.undo[1:])
		e.undo = e.undo[:maxUndoSnapshots-1]
	}
	e.undo = append(e.undo, snapshot)
}

func (e *Editor) pushUndo() {
	e.pushUndoSnapshot(e.snapshot())
}

func (e *Editor) Undo() bool {
	if len(e.undo) == 0 {
		return false
	}
	last := e.undo[len(e.undo)-1]
	e.undo = e.undo[:len(e.undo)-1]
	e.Content = cloneContent(last.content)
	e.Cursor = last.cursor
	e.scrollY = last.scrollY
	e.manualScroll = last.manualScroll
	e.MakeDirty()
	e.clamp()
	return true
}

func (e *Editor) GetContent() string {
	lines := make([]string, len(e.Content))
	for i, row := range e.Content {
		lines[i] = string(row)
	}
	return strings.Join(lines, "\n")
}

func (e *Editor) LastCursorUpdate(nextRow []rune) uint {
	if e.Cursor.lastCol > uint(len(nextRow)) {
		return uint(len(nextRow))
	}
	return e.Cursor.lastCol
}

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
	col := vl.start + x - (e.x + 4)
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

func (e *Editor) Scroll(delta int) bool {
	if e.h <= 0 {
		return false
	}

	before := e.scrollY
	e.scrollY -= delta
	e.clampScroll(e.lineCount)
	if e.scrollY == before {
		return false
	}
	e.manualScroll = true
	return true
}

func (e *Editor) ScrollPage(direction int) bool {
	page := e.h - 1
	if page < 1 {
		page = 1
	}
	return e.Scroll(direction * page)
}

func (e *Editor) ScrollTop() bool {
	if e.scrollY == 0 {
		return false
	}
	e.scrollY = 0
	e.manualScroll = true
	return true
}

func (e *Editor) ScrollBottom() bool {
	before := e.scrollY
	e.scrollY = e.lineCount - e.h
	e.clampScroll(e.lineCount)
	if e.scrollY == before {
		return false
	}
	e.manualScroll = true
	return true
}

// ── Layout / rendering ─────────────────────────────────────────────────────

func (e *Editor) contentWidth() int {
	w := e.w - 4
	if w < 1 {
		w = 1
	}
	return w
}

func (e *Editor) computeVisualLines() []editorVisualLine {
	var vl []editorVisualLine
	width := e.contentWidth()
	for row, line := range e.Content {
		lineLen := len(line)
		if lineLen == 0 {
			vl = append(vl, editorVisualLine{row: row, start: 0, end: 0})
			continue
		}
		for col := 0; col < lineLen; col += width {
			end := col + width
			if end > lineLen {
				end = lineLen
			}
			vl = append(vl, editorVisualLine{row: row, start: col, end: end})
		}
	}
	return vl
}

func (e *Editor) visualLineForCursor() int {
	width := e.contentWidth()
	vl := 0
	for row := 0; row < e.Cursor.Pos.Row; row++ {
		lineLen := len(e.Content[row])
		if lineLen == 0 {
			vl++
		} else {
			vl += (lineLen + width - 1) / width
		}
	}
	if width > 0 {
		vl += e.Cursor.Pos.Col / width
	}
	return vl
}

func (e *Editor) clampScroll(lineCount int) {
	if e.scrollY < 0 {
		e.scrollY = 0
	}
	maxScroll := lineCount - e.h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if e.scrollY > maxScroll {
		e.scrollY = maxScroll
	}
}

func (e *Editor) Render(s *screen.Screen) {
	fg := e.style.GetForeground()
	bg := e.style.GetBackground()
	decor := e.style.GetDecor()

	if e.parent != nil {
		ps := e.parent.GetStyle()
		bg = ps.GetBackground()
		fg = ps.GetForeground()
		decor = ps.GetDecor()
	}

	gutterW := 4
	contentW := e.contentWidth()

	visLines := e.computeVisualLines()
	e.lineCount = len(visLines)
	cursorVL := e.visualLineForCursor()

	if !e.manualScroll {
		if cursorVL < e.scrollY {
			e.scrollY = cursorVL
		}
		if cursorVL >= e.scrollY+e.h {
			e.scrollY = cursorVL - e.h + 1
		}
	}
	e.clampScroll(len(visLines))

	// ── Colour palette ──────────────────────────────────────────────────
	// Selection
	selFg := "\x1b[38;2;215;225;255m"
	selBg := "\x1b[48;2;45;80;158m"
	// Block cursor
	cursorFg := "\x1b[38;2;14;14;20m"
	cursorBg := "\x1b[48;2;165;188;255m"
	// Current-line highlight
	lineBg := "\x1b[48;2;40;38;50m"
	// Gutter
	gutterFg := "\x1b[38;2;72;70;84m"
	numFg := "\x1b[38;2;95;93;108m"
	activeNumFg := "\x1b[38;2;200;198;212m"

	prevRow := -1
	for i := 0; i < e.h; i++ {
		vlIdx := i + e.scrollY
		screenY := e.y + i

		if vlIdx >= len(visLines) {
			for x := 0; x < e.w; x++ {
				s.Set(e.x+x, screenY, ' ', fg, bg, decor)
			}
			continue
		}

		vl := visLines[vlIdx]
		line := e.Content[vl.row]
		isCurrentLine := vl.row == e.Cursor.Pos.Row
		isTheCursorVL := vlIdx == cursorVL
		isFirstSegment := vl.row != prevRow
		prevRow = vl.row

		rowBg := bg
		if isCurrentLine {
			rowBg = lineBg
		}

		// ── Gutter ────────────────────────────────────────────────────
		lineNumFg := numFg
		if isCurrentLine {
			lineNumFg = activeNumFg
		}
		if isFirstSegment {
			num := fmt.Sprintf("%3d", vl.row+1)
			for n, r := range num {
				s.Set(e.x+n, screenY, r, lineNumFg, rowBg, decor)
			}
		} else {
			for gx := 0; gx < 3; gx++ {
				s.Set(e.x+gx, screenY, ' ', fg, rowBg, decor)
			}
		}
		s.Set(e.x+3, screenY, '\u2502', gutterFg, rowBg, decor)

		// ── Content ───────────────────────────────────────────────────
		for x := 0; x < contentW; x++ {
			col := vl.start + x
			var ch rune = ' '
			cellFg := fg
			cellBg := rowBg

			if col < vl.end {
				ch = line[col]
			}

			if e.isSelected(vl.row, col) {
				cellFg, cellBg = selFg, selBg
			}

			if isTheCursorVL && col == e.Cursor.Pos.Col {
				cellFg, cellBg = cursorFg, cursorBg
			}

			s.Set(e.x+gutterW+x, screenY, ch, cellFg, cellBg, decor)
		}
	}

	cursorScreenVL := cursorVL - e.scrollY
	e.cursorScreenY = e.y + cursorScreenVL
	e.cursorScreenX = e.x + gutterW + (e.Cursor.Pos.Col % contentW)
}

func (e *Editor) RevealCursor() {
	e.manualScroll = false
}

func (e *Editor) CursorScreenPos() (int, int) {
	return e.cursorScreenX, e.cursorScreenY
}
