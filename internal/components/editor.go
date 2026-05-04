package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type CursorPos struct {
	Row int
	Col int
}

type Cursor struct {
	// Position of cursor on grid
	pos CursorPos

	// Use to track selection of text in the editor
	selStart           CursorPos
	selEnd             CursorPos
	isSelectionStarted bool

	// Use to remember cursor position on shorter lines
	lastCol uint
}

type Editor struct {
	isDirty bool

	Content [][]rune
	Cursor  Cursor

	style  layout.Style
	parent layout.Component

	x, y int
	w, h int

	scrollY int
	scrollX int
}

func NewTextEditor() *Editor {
	return &Editor{
		Content: [][]rune{},
		Cursor:  Cursor{},
		style:   layout.Style{},
		parent:  nil,
		x:       0,
		y:       0,
		w:       0,
		h:       0,
		scrollY: 0,
		scrollX: 0,
	}
}

func (e *Editor) IsDirty() bool                { return e.isDirty }
func (e *Editor) MakeDirty()                   { e.isDirty = true }
func (e *Editor) ClearDirty()                  { e.isDirty = false }
func (e *Editor) GetStyle() layout.Style       { return e.style }
func (e *Editor) SetParent(p layout.Component) { e.parent = p }
func (e *Editor) SetStyle(s layout.Style)      { e.style = s }

func (e *Editor) Layout(x, y, w, h int)        {}
func (e *Editor) Render(screen *screen.Screen) {}

func (e *Editor) CursorMoveRowUp() {
	if e.Cursor.pos.Row == 0 {
		e.Cursor.pos.Col = 0
	} else {
		e.Cursor.pos.Row--
		e.Cursor.pos.Col = int(e.LastCursorUpdate(e.Content[e.Cursor.pos.Row]))
	}
}
func (e *Editor) CursorMoveRowDown() {
	if e.Cursor.pos.Row != len(e.Content)-1 {
		e.Cursor.pos.Row++
		e.Cursor.pos.Col = int(e.Cursor.lastCol)
		e.Cursor.pos.Col = int(e.LastCursorUpdate(e.Content[e.Cursor.pos.Row]))
	}
}
func (e *Editor) CursorMoveColRight() {
	if e.Cursor.pos.Col == len(e.Content[e.Cursor.pos.Row]) {
		e.CursorMoveRowDown()
		e.Cursor.pos.Col = 0
	} else {
		e.Cursor.pos.Col++
		// Deal with the types ... Do i need lastCol to be uint or do i need pos to be ints
		e.Cursor.lastCol = uint(e.Cursor.pos.Col)
	}
}
func (e *Editor) CursorMoveColLeft() {
	if e.Cursor.pos.Col == 0 {
		e.CursorMoveRowUp()
	} else {
		e.Cursor.pos.Col--
		e.Cursor.lastCol = uint(e.Cursor.pos.Col)
	}
}

func (e *Editor) LastCursorUpdate(nextRow []rune) uint {
	if e.Cursor.lastCol > uint(len(nextRow)) {
		return uint(len(nextRow))
	}
	return e.Cursor.lastCol
}

func (e *Editor) ShiftPress() {
	e.Cursor.isSelectionStarted = true
	e.Cursor.selStart = e.Cursor.pos
}

func (e *Editor) ShiftRelease() {
	e.Cursor.isSelectionStarted = false
}

func (e *Editor) AppendAfterCursor(r rune) {
	e.Content[e.Cursor.pos.Row] = append(e.Content[e.Cursor.pos.Row], r)
	e.Cursor.pos.Col++
}

func (e *Editor) RemoveBeforeCursor() {
	if e.Cursor.pos.Col > 0 {
		e.Content[e.Cursor.pos.Row] = append(e.Content[e.Cursor.pos.Row][:e.Cursor.pos.Col-1], e.Content[e.Cursor.pos.Row][e.Cursor.pos.Col:]...)
		e.Cursor.pos.Col--
	}
}

func (e *Editor) InsertNewLine() {
	e.Content = append(e.Content, make([]rune, 0))
	e.Cursor.pos.Row++
	e.Cursor.pos.Col = 0
}
