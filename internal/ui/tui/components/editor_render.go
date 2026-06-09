package components

import (
	"fmt"

	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

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
		s.Set(e.x+3, screenY, '│', gutterFg, rowBg, decor)

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
