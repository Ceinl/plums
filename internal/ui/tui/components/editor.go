package components

import (
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
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
	inputBoxMode   bool
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
	e.inputBoxMode = false
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
