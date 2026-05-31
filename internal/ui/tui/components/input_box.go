package components

import (
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

type InputBox struct {
	ed     *Editor
	style  layout.Style
	parent layout.Component
	x, y   int
	w, h   int
}

func NewInputBox(ed *Editor) *InputBox {
	return &InputBox{ed: ed}
}

func (b *InputBox) IsDirty() bool { return b.ed != nil && b.ed.IsDirty() }
func (b *InputBox) MakeDirty() {
	if b.ed != nil {
		b.ed.MakeDirty()
	}
}
func (b *InputBox) ClearDirty() {
	if b.ed != nil {
		b.ed.ClearDirty()
	}
}
func (b *InputBox) GetStyle() layout.Style       { return b.style }
func (b *InputBox) SetParent(p layout.Component) { b.parent = p }
func (b *InputBox) SetStyle(s layout.Style)      { b.style = s }

func (b *InputBox) Layout(x, y, w, h int) {
	b.x, b.y, b.w, b.h = x, y, w, h
	if b.ed != nil {
		b.ed.Layout(x, y, w, h)
		b.ed.inputBoxMode = true
	}
}

func (b *InputBox) Render(s *screen.Screen) {
	if b.ed == nil {
		return
	}

	ed := b.ed
	fg := b.style.GetForeground()
	bg := b.style.GetBackground()
	decor := b.style.GetDecor()
	if b.parent != nil {
		ps := b.parent.GetStyle()
		fg = ps.GetForeground()
		bg = ps.GetBackground()
		decor = ps.GetDecor()
	}

	contentW := b.w - 2
	if contentW < 1 {
		contentW = 1
	}
	visLines := inputBoxVisualLines(ed, contentW)
	ed.lineCount = len(visLines)
	cursorVL := inputBoxVisualLineForCursor(ed, contentW)
	if !ed.manualScroll {
		if cursorVL < ed.scrollY {
			ed.scrollY = cursorVL
		}
		if cursorVL >= ed.scrollY+b.h {
			ed.scrollY = cursorVL - b.h + 1
		}
	}
	ed.clampScroll(len(visLines))

	selFg := "\x1b[38;2;22;20;27m"
	selBg := "\x1b[48;2;200;198;212m"
	cursorFg := "\x1b[38;2;14;14;20m"
	cursorBg := "\x1b[48;2;183;255;90m"
	lineBg := "\x1b[48;2;28;31;29m"
	borderFg := "\x1b[38;2;95;105;96m"

	for row := 0; row < b.h; row++ {
		screenY := b.y + row
		vlIdx := row + ed.scrollY
		rowBg := bg
		if vlIdx == cursorVL {
			rowBg = lineBg
		}
		for col := 0; col < b.w; col++ {
			s.Set(b.x+col, screenY, ' ', fg, rowBg, decor)
		}
		if b.w > 0 {
			s.Set(b.x, screenY, '>', borderFg, rowBg, decor)
		}
		if vlIdx >= len(visLines) {
			continue
		}

		vl := visLines[vlIdx]
		line := ed.Content[vl.row]
		for col := 0; col < contentW; col++ {
			contentCol := vl.start + col
			ch := ' '
			cellFg := fg
			cellBg := rowBg
			if contentCol < vl.end {
				ch = line[contentCol]
			}
			if ed.isSelected(vl.row, contentCol) {
				cellFg, cellBg = selFg, selBg
			}
			if vlIdx == cursorVL && contentCol == ed.Cursor.Pos.Col {
				cellFg, cellBg = cursorFg, cursorBg
			}
			s.Set(b.x+1+col, screenY, ch, cellFg, cellBg, decor)
		}
	}

	ed.cursorScreenY = b.y + cursorVL - ed.scrollY
	ed.cursorScreenX = b.x + 1 + (ed.Cursor.Pos.Col % contentW)
}

func inputBoxVisualLines(ed *Editor, width int) []editorVisualLine {
	var lines []editorVisualLine
	for row, line := range ed.Content {
		lineLen := len(line)
		if lineLen == 0 {
			lines = append(lines, editorVisualLine{row: row, start: 0, end: 0})
			continue
		}
		for col := 0; col < lineLen; col += width {
			end := col + width
			if end > lineLen {
				end = lineLen
			}
			lines = append(lines, editorVisualLine{row: row, start: col, end: end})
		}
	}
	return lines
}

func inputBoxVisualLineForCursor(ed *Editor, width int) int {
	vl := 0
	for row := 0; row < ed.Cursor.Pos.Row; row++ {
		lineLen := len(ed.Content[row])
		if lineLen == 0 {
			vl++
		} else {
			vl += (lineLen + width - 1) / width
		}
	}
	vl += ed.Cursor.Pos.Col / width
	return vl
}
