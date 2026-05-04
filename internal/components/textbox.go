package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type textLine struct {
	start, end int // rune indices
}

type TextBox struct {
	isDirty bool

	content    []rune
	cursor     int
	selStart   int
	selEnd     int
	desiredCol int
	multiline  bool

	parent layout.Component
	x, y   int
	w, h   int
	style  layout.Style

	scrollX int
	scrollY int

	cursorScreenX int
	cursorScreenY int
}

func NewTextBox() *TextBox {
	return &TextBox{
		selStart: -1,
	}
}

func (tb *TextBox) IsDirty() bool                { return tb.isDirty }
func (tb *TextBox) MakeDirty()                   { tb.isDirty = true }
func (tb *TextBox) ClearDirty()                  { tb.isDirty = false }
func (tb *TextBox) GetStyle() layout.Style       { return tb.style }
func (tb *TextBox) SetParent(p layout.Component) { tb.parent = p }
func (tb *TextBox) SetStyle(s layout.Style)      { tb.style = s }

func (tb *TextBox) SetMultiline(v bool) { tb.multiline = v }
func (tb *TextBox) IsMultiline() bool   { return tb.multiline }

func (tb *TextBox) SetContent(s string) {
	if string(tb.content) != s {
		tb.content = []rune(s)
		tb.isDirty = true
	}
}

func (tb *TextBox) GetContent() string {
	return string(tb.content)
}

func (tb *TextBox) CursorPos() int {
	return tb.cursor
}

func (tb *TextBox) SetCursor(pos int) {
	if pos < 0 {
		pos = 0
	}
	if pos > len(tb.content) {
		pos = len(tb.content)
	}
	tb.cursor = pos
}

func (tb *TextBox) HasSelection() bool {
	return tb.selStart >= 0 && tb.selStart < tb.selEnd
}

func (tb *TextBox) SelectedText() string {
	if !tb.HasSelection() {
		return ""
	}
	return string(tb.content[tb.selStart:tb.selEnd])
}

func (tb *TextBox) ClearSelection() {
	tb.selStart = -1
	tb.selEnd = -1
}

func (tb *TextBox) normalizeSelection() {
	if tb.selStart < 0 {
		tb.selStart = -1
		tb.selEnd = -1
		return
	}
	if tb.selStart > tb.selEnd {
		tb.selStart, tb.selEnd = tb.selEnd, tb.selStart
	}
	if tb.selStart < 0 {
		tb.selStart = 0
	}
	if tb.selEnd > len(tb.content) {
		tb.selEnd = len(tb.content)
	}
	if tb.selStart >= tb.selEnd {
		tb.selStart = -1
		tb.selEnd = -1
	}
}

func (tb *TextBox) isSelected(idx int) bool {
	if tb.selStart < 0 {
		return false
	}
	return idx >= tb.selStart && idx < tb.selEnd
}

// --- Line computation ---

func (tb *TextBox) computeLines() []textLine {
	if tb.multiline {
		return tb.computeWrappedLines()
	}
	return []textLine{{start: 0, end: len(tb.content)}}
}

func (tb *TextBox) computeWrappedLines() []textLine {
	if tb.w <= 0 {
		return []textLine{{start: 0, end: len(tb.content)}}
	}
	var lines []textLine
	start := 0
	col := 0
	for i := 0; i < len(tb.content); i++ {
		if tb.content[i] == '\n' {
			lines = append(lines, textLine{start: start, end: i})
			start = i + 1
			col = 0
			continue
		}
		if col >= tb.w {
			lines = append(lines, textLine{start: start, end: i})
			start = i
			col = 0
		}
		col++
	}
	lines = append(lines, textLine{start: start, end: len(tb.content)})
	return lines
}

func (tb *TextBox) lineIndexForCursor(lines []textLine) int {
	if len(lines) == 0 {
		return 0
	}
	for i, line := range lines {
		if tb.cursor >= line.start && tb.cursor <= line.end {
			return i
		}
	}
	return len(lines) - 1
}

// --- Cursor movement ---

func (tb *TextBox) MoveCursorLeft() {
	if tb.HasSelection() {
		tb.cursor = tb.selStart
		tb.ClearSelection()
		return
	}
	if tb.cursor > 0 {
		tb.cursor--
	}
	tb.updateDesiredCol()
}

func (tb *TextBox) MoveCursorRight() {
	if tb.HasSelection() {
		tb.cursor = tb.selEnd
		tb.ClearSelection()
		return
	}
	if tb.cursor < len(tb.content) {
		tb.cursor++
	}
	tb.updateDesiredCol()
}

func (tb *TextBox) MoveCursorUp() {
	if !tb.multiline {
		tb.MoveCursorHome()
		return
	}
	lines := tb.computeLines()
	li := tb.lineIndexForCursor(lines)
	if li == 0 {
		tb.cursor = 0
		tb.updateDesiredCol()
		return
	}
	prev := lines[li-1]
	off := tb.cursor - lines[li].start
	if off > prev.end-prev.start {
		off = prev.end - prev.start
	}
	tb.cursor = prev.start + off
	tb.updateDesiredCol()
}

func (tb *TextBox) MoveCursorDown() {
	if !tb.multiline {
		tb.MoveCursorEnd()
		return
	}
	lines := tb.computeLines()
	li := tb.lineIndexForCursor(lines)
	if li >= len(lines)-1 {
		tb.cursor = len(tb.content)
		tb.updateDesiredCol()
		return
	}
	next := lines[li+1]
	off := tb.desiredCol
	if off > next.end-next.start {
		off = next.end - next.start
	}
	tb.cursor = next.start + off
}

func (tb *TextBox) MoveCursorHome() {
	if !tb.multiline {
		tb.cursor = 0
		tb.updateDesiredCol()
		return
	}
	lines := tb.computeLines()
	li := tb.lineIndexForCursor(lines)
	tb.cursor = lines[li].start
	tb.updateDesiredCol()
}

func (tb *TextBox) MoveCursorEnd() {
	if !tb.multiline {
		tb.cursor = len(tb.content)
		tb.updateDesiredCol()
		return
	}
	lines := tb.computeLines()
	li := tb.lineIndexForCursor(lines)
	tb.cursor = lines[li].end
	tb.updateDesiredCol()
}

func (tb *TextBox) updateDesiredCol() {
	lines := tb.computeLines()
	li := tb.lineIndexForCursor(lines)
	if li >= 0 && li < len(lines) {
		tb.desiredCol = tb.cursor - lines[li].start
	}
}

// --- Selection ---

func (tb *TextBox) beginSelectionIfNeeded() {
	if tb.selStart < 0 {
		tb.selStart = tb.cursor
		tb.selEnd = tb.cursor
	}
}

func (tb *TextBox) SelectLeft() {
	tb.beginSelectionIfNeeded()
	if tb.cursor > 0 {
		tb.cursor--
		tb.selEnd = tb.cursor
		tb.normalizeSelection()
	}
}

func (tb *TextBox) SelectRight() {
	tb.beginSelectionIfNeeded()
	if tb.cursor < len(tb.content) {
		tb.cursor++
		tb.selEnd = tb.cursor
		tb.normalizeSelection()
	}
}

func (tb *TextBox) SelectUp() {
	tb.beginSelectionIfNeeded()
	oldCursor := tb.cursor
	tb.MoveCursorUp()
	if tb.cursor != oldCursor {
		tb.selEnd = tb.cursor
		tb.normalizeSelection()
	}
}

func (tb *TextBox) SelectDown() {
	tb.beginSelectionIfNeeded()
	oldCursor := tb.cursor
	tb.MoveCursorDown()
	if tb.cursor != oldCursor {
		tb.selEnd = tb.cursor
		tb.normalizeSelection()
	}
}

func (tb *TextBox) SelectHome() {
	tb.beginSelectionIfNeeded()
	oldCursor := tb.cursor
	tb.MoveCursorHome()
	if tb.cursor != oldCursor {
		tb.selEnd = tb.cursor
		tb.normalizeSelection()
	}
}

func (tb *TextBox) SelectEnd() {
	tb.beginSelectionIfNeeded()
	oldCursor := tb.cursor
	tb.MoveCursorEnd()
	if tb.cursor != oldCursor {
		tb.selEnd = tb.cursor
		tb.normalizeSelection()
	}
}

func (tb *TextBox) SelectAll() {
	if len(tb.content) == 0 {
		tb.ClearSelection()
		return
	}
	tb.selStart = 0
	tb.selEnd = len(tb.content)
	tb.cursor = len(tb.content)
}

// --- Editing ---

func (tb *TextBox) InsertRune(r rune) {
	if tb.HasSelection() {
		tb.DeleteSelection()
	}
	if r == '\n' && !tb.multiline {
		return
	}
	if tb.cursor > len(tb.content) {
		tb.cursor = len(tb.content)
	}
	tb.content = append(tb.content[:tb.cursor], append([]rune{r}, tb.content[tb.cursor:]...)...)
	tb.cursor++
	tb.isDirty = true
}

func (tb *TextBox) InsertNewline() {
	if !tb.multiline {
		return
	}
	tb.InsertRune('\n')
}

func (tb *TextBox) DeleteBackward() {
	if tb.HasSelection() {
		tb.DeleteSelection()
		return
	}
	if tb.cursor > 0 {
		tb.content = append(tb.content[:tb.cursor-1], tb.content[tb.cursor:]...)
		tb.cursor--
		tb.isDirty = true
	}
}

func (tb *TextBox) DeleteForward() {
	if tb.HasSelection() {
		tb.DeleteSelection()
		return
	}
	if tb.cursor < len(tb.content) {
		tb.content = append(tb.content[:tb.cursor], tb.content[tb.cursor+1:]...)
		tb.isDirty = true
	}
}

func (tb *TextBox) DeleteSelection() {
	if !tb.HasSelection() {
		return
	}
	tb.content = append(tb.content[:tb.selStart], tb.content[tb.selEnd:]...)
	tb.cursor = tb.selStart
	tb.ClearSelection()
	tb.isDirty = true
}

// --- Layout / Render ---

func (tb *TextBox) Layout(x, y, w, h int) {
	tb.x = x
	tb.y = y
	tb.w = w
	tb.h = h
}

func (tb *TextBox) Render(s *screen.Screen) {
	fg := tb.style.GetForeground()
	bg := tb.style.GetBackground()
	decor := tb.style.GetDecor()

	if tb.parent != nil {
		ps := tb.parent.GetStyle()
		bg = ps.GetBackground()
		fg = ps.GetForeground()
		decor = ps.GetDecor()
	}

	lines := tb.computeLines()
	li := tb.lineIndexForCursor(lines)

	// Adjust scroll to keep cursor visible
	if tb.multiline {
		if li < tb.scrollY {
			tb.scrollY = li
		}
		if li >= tb.scrollY+tb.h {
			tb.scrollY = li - tb.h + 1
		}
		if tb.scrollY < 0 {
			tb.scrollY = 0
		}
		if tb.scrollY > len(lines)-tb.h {
			if len(lines) > tb.h {
				tb.scrollY = len(lines) - tb.h
			} else {
				tb.scrollY = 0
			}
		}
	} else {
		tb.scrollY = 0
		// Horizontal scroll for single-line
		col := tb.cursor
		if col < tb.scrollX {
			tb.scrollX = col
		}
		if col >= tb.scrollX+tb.w {
			tb.scrollX = col - tb.w + 1
		}
		if tb.scrollX < 0 {
			tb.scrollX = 0
		}
	}

	// Render visible lines
	for i := 0; i < tb.h; i++ {
		lineIdx := i + tb.scrollY
		screenY := tb.y + i
		if screenY < tb.y || screenY >= tb.y+tb.h {
			continue
		}
		if lineIdx >= len(lines) {
			// Fill remaining with background
			for xx := 0; xx < tb.w; xx++ {
				s.Set(tb.x+xx, screenY, ' ', fg, bg, decor)
			}
			continue
		}

		line := lines[lineIdx]
		xOff := 0
		if !tb.multiline {
			xOff = tb.scrollX
		}

		for xx := 0; xx < tb.w; xx++ {
			idx := line.start + xx + xOff
			var ch rune = ' '
			cellFg := fg
			cellBg := bg
			if idx >= line.start && idx < line.end && idx < len(tb.content) {
				ch = tb.content[idx]
			}
			if tb.isSelected(idx) {
				cellFg, cellBg = bg, fg
			}
			s.Set(tb.x+xx, screenY, ch, cellFg, cellBg, decor)
		}
	}

	// Compute cursor screen position
	if tb.multiline {
		cursorLine := li
		tb.cursorScreenY = tb.y + cursorLine - tb.scrollY
		tb.cursorScreenX = tb.x
		if cursorLine >= 0 && cursorLine < len(lines) {
			off := tb.cursor - lines[cursorLine].start
			if off < 0 {
				off = 0
			}
			if off > tb.w {
				off = tb.w
			}
			tb.cursorScreenX = tb.x + off
		}
	} else {
		tb.cursorScreenY = tb.y
		tb.cursorScreenX = tb.x + tb.cursor - tb.scrollX
	}
}

func (tb *TextBox) CursorScreenPos() (int, int) {
	return tb.cursorScreenX, tb.cursorScreenY
}
