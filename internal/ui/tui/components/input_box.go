package components

import (
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// StatusSegment is one coloured fragment of the status line shown above the
// input panel. An empty Fg falls back to the default faint status colour.
type StatusSegment struct {
	Text string
	Fg   string
}

type InputBox struct {
	ed         *Editor
	style      layout.Style
	parent     layout.Component
	x, y       int
	w, h       int
	status     []StatusSegment
	statusText string // cached plain text, for width handling

	// floating panel geometry (computed in Layout)
	px, py int // panel top-left
	pw, ph int // panel size
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

func (b *InputBox) SetStatus(s string) {
	b.SetStatusSegments([]StatusSegment{{Text: s}})
}

func (b *InputBox) SetStatusSegments(segs []StatusSegment) {
	b.status = segs
	b.statusText = ""
	for _, seg := range segs {
		b.statusText += seg.Text
	}
}

func (b *InputBox) Layout(x, y, w, h int) {
	b.x, b.y, b.w, b.h = x, y, w, h

	// Desired panel height based on content lines (+2 for top/bottom border)
	contentLines := 1
	if b.ed != nil {
		contentLines = len(b.ed.Content)
	}
	desiredPh := contentLines + 2
	if desiredPh < 3 {
		desiredPh = 3
	}
	maxPh := h - 1 // 1-row bottom margin
	if maxPh < 3 {
		maxPh = h
	}
	b.ph = desiredPh
	if b.ph > maxPh {
		b.ph = maxPh
	}

	b.pw = int(float64(w) * 0.8)
	if b.pw < 10 {
		b.pw = w
	}
	b.px = x + (w-b.pw)/2
	// Anchor panel to bottom of wrapper so it grows upward
	b.py = y + h - b.ph - 1 // -1 for bottom margin
	if b.py < y {
		b.py = y
	}

	if b.ed != nil {
		textRows := b.ph - 2
		if textRows < 1 {
			textRows = 1
		}
		lineX := b.px + 2
		lineW := b.pw - 4
		if b.pw < 8 {
			lineX = b.px
			lineW = b.pw
		}
		if lineW < 1 {
			lineW = 1
		}
		lineY := b.py + 1
		b.ed.Layout(lineX-1, lineY, lineW+4, textRows)
		b.ed.inputBoxMode = true
	}
}

func (b *InputBox) Render(s *screen.Screen) {
	if b.ed == nil {
		return
	}

	ed := b.ed
	fg := b.style.GetForeground()
	decor := b.style.GetDecor()
	parentBg := theme.Unset.Bg()
	if b.parent != nil {
		ps := b.parent.GetStyle()
		fg = ps.GetForeground()
		parentBg = ps.GetBackground()
		decor = ps.GetDecor()
	}

	selFg := theme.SelectionFg.Fg()
	selBg := theme.SelectionBg.Bg()
	cursorFg := theme.CursorFg.Fg()
	cursorBg := theme.CursorBg.Bg()
	lineBg := parentBg
	borderFg := theme.BorderAccent.Fg()

	// Fill entire allocated area with parent background (transparent edges)
	for row := 0; row < b.h; row++ {
		screenY := b.y + row
		for col := 0; col < b.w; col++ {
			s.Set(b.x+col, screenY, ' ', fg, parentBg, decor)
		}
	}

	// Draw status text just above the panel, moving with it
	if b.statusText != "" && b.py > b.y {
		statusFg := theme.TextFaint.Fg()
		statusY := b.py - 1
		// clear the row inside the 80% width area
		for col := 0; col < b.pw; col++ {
			s.Set(b.px+col, statusY, ' ', statusFg, parentBg, decor)
		}
		col := 0
		for _, seg := range b.status {
			segFg := seg.Fg
			if segFg == "" {
				segFg = statusFg
			}
			for _, r := range seg.Text {
				if col >= b.pw {
					break
				}
				s.Set(b.px+col, statusY, r, segFg, parentBg, decor)
				col++
			}
		}
	}

	// Draw rounded border if panel is large enough
	if b.pw >= 4 && b.ph >= 3 {
		s.Set(b.px, b.py, '╭', borderFg, parentBg, decor)
		s.Set(b.px+b.pw-1, b.py, '╮', borderFg, parentBg, decor)
		s.Set(b.px, b.py+b.ph-1, '╰', borderFg, parentBg, decor)
		s.Set(b.px+b.pw-1, b.py+b.ph-1, '╯', borderFg, parentBg, decor)
		for col := 1; col < b.pw-1; col++ {
			s.Set(b.px+col, b.py, '─', borderFg, parentBg, decor)
			s.Set(b.px+col, b.py+b.ph-1, '─', borderFg, parentBg, decor)
		}
		for row := 1; row < b.ph-1; row++ {
			s.Set(b.px, b.py+row, '│', borderFg, parentBg, decor)
			s.Set(b.px+b.pw-1, b.py+row, '│', borderFg, parentBg, decor)
		}
	}

	ed.clamp()

	textRows := b.ph - 2
	if textRows < 1 {
		textRows = 1
	}
	ed.lineCount = len(ed.Content)
	ed.h = textRows

	// Ensure cursor is visible (anchor to bottom: show last lines)
	cursorRow := ed.Cursor.Pos.Row
	if cursorRow < ed.scrollY {
		ed.scrollY = cursorRow
	}
	if cursorRow >= ed.scrollY+textRows {
		ed.scrollY = cursorRow - textRows + 1
	}
	if ed.scrollY < 0 {
		ed.scrollY = 0
	}
	maxScroll := len(ed.Content) - textRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ed.scrollY > maxScroll {
		ed.scrollY = maxScroll
	}

	lineX := b.px + 2
	lineW := b.pw - 4
	if b.pw < 8 {
		lineX = b.px
		lineW = b.pw
	}
	if lineW < 1 {
		lineW = 1
	}
	textStartY := b.py + 1

	// Render each visible line
	for row := 0; row < textRows; row++ {
		contentRow := ed.scrollY + row
		screenY := textStartY + row

		// Fill line background with black inside the panel
		for col := 0; col < lineW; col++ {
			s.Set(lineX+col, screenY, ' ', fg, lineBg, decor)
		}

		if contentRow >= len(ed.Content) {
			continue
		}

		line := ed.Content[contentRow]
		cursorCol := -1
		if contentRow == ed.Cursor.Pos.Row {
			cursorCol = ed.Cursor.Pos.Col
		}

		// Horizontal offset for long lines
		offset := 0
		if cursorCol >= lineW {
			offset = cursorCol - lineW + 1
		}

		for col := 0; col < lineW; col++ {
			contentCol := offset + col
			ch := ' '
			if contentCol < len(line) {
				ch = line[contentCol]
			}

			cellFg := fg
			cellBg := lineBg
			if ed.isSelected(contentRow, contentCol) {
				cellFg, cellBg = selFg, selBg
			}
			if contentCol == cursorCol {
				cellFg, cellBg = cursorFg, cursorBg
			}
			s.Set(lineX+col, screenY, ch, cellFg, cellBg, decor)
		}
	}

	// Update editor's cursor screen position
	cursorRow = ed.Cursor.Pos.Row
	cursorCol := ed.Cursor.Pos.Col
	cursorScreenRow := cursorRow - ed.scrollY
	if cursorScreenRow < 0 {
		cursorScreenRow = 0
	}
	if cursorScreenRow >= textRows {
		cursorScreenRow = textRows - 1
	}

	offset := 0
	if cursorCol >= lineW {
		offset = cursorCol - lineW + 1
	}
	cursorX := lineX + cursorCol - offset
	if cursorX < lineX {
		cursorX = lineX
	}
	if cursorX >= lineX+lineW {
		cursorX = lineX + lineW - 1
	}
	ed.cursorScreenY = textStartY + cursorScreenRow
	ed.cursorScreenX = cursorX
}
