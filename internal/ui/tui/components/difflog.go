package components

import (
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

var (
	diffFgFile   = decorBold + theme.DiffFile.Fg()
	diffFgHunk   = theme.DiffHunk.Fg()
	diffFgAdd    = theme.DiffAddFg.Fg()
	diffFgRemove = theme.DiffRemoveFg.Fg()
	diffBgAdd    = theme.DiffAddBg.Bg()
	diffBgRemove = theme.DiffRemoveBg.Bg()

	diffFgDefault = theme.TextSoft.Fg()
	diffFgMeta    = theme.TextFaint.Fg()

	diffSelFg = theme.SelectionFg.Fg()
	diffSelBg = theme.SelectionBg.Bg()
)

type DiffLog struct {
	isDirty      bool
	content      string
	scrollOffset int
	onMaxScroll  func(int)
	linesCached  bool
	cachedLines  []string
	parent       layout.Component
	x, y         int
	w, h         int
	style        layout.Style

	// mouse selection
	selActive            bool
	selStartX, selStartY int
	selEndX, selEndY     int
}

func NewDiffLog() *DiffLog { return &DiffLog{} }

func (d *DiffLog) SetContent(content string) {
	if d.content == content {
		return
	}
	d.content = content
	d.invalidateLines()
	d.ClearSelection()
	d.isDirty = true
}

func (d *DiffLog) SetScrollOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	if d.scrollOffset == offset {
		return
	}
	d.scrollOffset = offset
	d.ClearSelection()
	d.isDirty = true
}

func (d *DiffLog) SetMaxScrollObserver(fn func(int)) {
	d.onMaxScroll = fn
}

func (d *DiffLog) IsDirty() bool                { return d.isDirty }
func (d *DiffLog) MakeDirty()                   { d.isDirty = true }
func (d *DiffLog) ClearDirty()                  { d.isDirty = false }
func (d *DiffLog) GetStyle() layout.Style       { return d.style }
func (d *DiffLog) SetParent(p layout.Component) { d.parent = p }
func (d *DiffLog) SetStyle(st layout.Style)     { d.style = st }

func (d *DiffLog) Layout(x, y, w, h int) {
	d.x, d.y, d.w, d.h = x, y, w, h
}

func (d *DiffLog) MouseDown(x, y int) bool {
	if x < d.x || x >= d.x+d.w || y < d.y || y >= d.y+d.h {
		return false
	}
	d.selActive = true
	d.selStartX, d.selStartY = x, y
	d.selEndX, d.selEndY = x, y
	d.isDirty = true
	return true
}

func (d *DiffLog) MouseDrag(x, y int) {
	if !d.selActive {
		return
	}
	d.selEndX, d.selEndY = x, y
	d.isDirty = true
}

func (d *DiffLog) MouseUp(x, y int) string {
	if !d.selActive {
		return ""
	}
	d.selEndX, d.selEndY = x, y
	text := d.extractSelection()
	d.selActive = false
	d.isDirty = true
	return text
}

func (d *DiffLog) ClearSelection() {
	d.selActive = false
	d.isDirty = true
}

func (d *DiffLog) rowSelectionRange(screenY int) (startX, endX int, ok bool) {
	if !d.selActive {
		return 0, 0, false
	}
	sy1, sy2 := d.selStartY, d.selEndY
	if sy1 > sy2 {
		sy1, sy2 = sy2, sy1
	}
	if screenY < sy1 || screenY > sy2 {
		return 0, 0, false
	}

	sx1, sx2 := d.selStartX, d.selEndX
	if sy1 != sy2 {
		if d.selStartY <= d.selEndY {
			if screenY == d.selStartY {
				return sx1, d.x + d.w, true
			}
			if screenY == d.selEndY {
				return d.x, sx2, true
			}
			return d.x, d.x + d.w, true
		}
		// dragged upward
		if screenY == d.selEndY {
			return sx2, d.x + d.w, true
		}
		if screenY == d.selStartY {
			return d.x, sx1, true
		}
		return d.x, d.x + d.w, true
	}
	if sx1 > sx2 {
		sx1, sx2 = sx2, sx1
	}
	return sx1, sx2, true
}

func (d *DiffLog) extractSelection() string {
	if !d.selActive {
		return ""
	}

	lines := d.lines()
	maxScroll := len(lines) - d.h
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollOffset := d.scrollOffset
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	start := 0
	if len(lines) > d.h {
		maxStart := len(lines) - d.h
		start = maxStart - scrollOffset
		if start < 0 {
			start = 0
		}
	}

	var out []string
	for row := 0; row < d.h; row++ {
		y := d.y + row
		idx := start + row
		if idx >= len(lines) {
			continue
		}

		rx1, rx2, ok := d.rowSelectionRange(y)
		if !ok {
			continue
		}

		line := lines[idx]
		var runes []rune
		curX := d.x
		// indent
		for i := 0; i < 2 && curX < d.x+d.w; i++ {
			if curX >= rx1 && curX < rx2 {
				runes = append(runes, ' ')
			}
			curX++
		}
		for _, r := range line {
			if r == '\t' {
				spaces := 4 - ((curX - d.x - 2) % 4)
				for i := 0; i < spaces && curX < d.x+d.w; i++ {
					if curX >= rx1 && curX < rx2 {
						runes = append(runes, ' ')
					}
					curX++
				}
				continue
			}
			if curX >= rx1 && curX < rx2 {
				runes = append(runes, r)
			}
			curX++
		}
		if len(runes) > 0 {
			out = append(out, string(runes))
		}
	}

	return strings.Join(out, "\n")
}

func (d *DiffLog) selectionFgBg(y, x int, fg, bg string) (string, string) {
	rx1, rx2, ok := d.rowSelectionRange(y)
	if !ok {
		return fg, bg
	}
	if x >= rx1 && x < rx2 {
		return diffSelFg, diffSelBg
	}
	return fg, bg
}

func (d *DiffLog) MaxScrollOffset() int {
	maxOffset := len(d.lines()) - d.h
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (d *DiffLog) Render(scr *screen.Screen) {
	if d.w <= 0 || d.h <= 0 {
		return
	}

	bg := d.style.GetBackground()
	if d.parent != nil {
		bg = d.parent.GetStyle().GetBackground()
	}

	lines := d.lines()
	maxScroll := len(lines) - d.h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.onMaxScroll != nil {
		d.onMaxScroll(maxScroll)
	}
	scrollOffset := d.scrollOffset
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	start := 0
	if len(lines) > d.h {
		maxStart := len(lines) - d.h
		start = maxStart - scrollOffset
		if start < 0 {
			start = 0
		}
	}

	for row := 0; row < d.h; row++ {
		y := d.y + row
		idx := start + row
		if idx >= len(lines) {
			d.clearRow(scr, y, bg)
			continue
		}
		d.renderLine(scr, y, lines[idx], bg)
	}
}

func (d *DiffLog) lines() []string {
	if d.linesCached {
		return d.cachedLines
	}

	var out []string
	if strings.TrimSpace(d.content) == "" {
		out = []string{"No git diff."}
	} else {
		for _, line := range strings.Split(d.content, "\n") {
			switch {
			case strings.HasPrefix(line, "diff --git "):
				out = append(out, "")
			case strings.HasPrefix(line, "index "):
				continue
			case strings.HasPrefix(line, "--- "):
				continue
			case strings.HasPrefix(line, "+++ b/"):
				out = append(out, strings.TrimPrefix(line, "+++ b/"))
			case strings.HasPrefix(line, "+++ "):
				out = append(out, strings.TrimPrefix(line, "+++ "))
			default:
				out = append(out, line)
			}
		}
	}
	d.cachedLines = out
	d.linesCached = true
	return d.cachedLines
}

func (d *DiffLog) invalidateLines() {
	d.linesCached = false
	d.cachedLines = nil
}

func (d *DiffLog) renderLine(scr *screen.Screen, y int, line string, bg string) {
	fg := diffFgDefault
	lineBg := bg
	trimmed := strings.TrimSpace(line)

	switch {
	case trimmed == "":
		fg = diffFgMeta
	case strings.HasPrefix(line, "@@"):
		fg = diffFgHunk
	case strings.HasPrefix(line, "+"):
		fg = diffFgAdd
		lineBg = diffBgAdd
	case strings.HasPrefix(line, "-"):
		fg = diffFgRemove
		lineBg = diffBgRemove
	case !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t"):
		fg = diffFgFile
	}

	x := d.x
	for i := 0; i < 2 && x < d.x+d.w; i++ {
		cellFg, cellBg := d.selectionFgBg(y, x, fg, lineBg)
		scr.Set(x, y, ' ', cellFg, cellBg, "")
		x++
	}
	for _, r := range line {
		if x >= d.x+d.w {
			break
		}
		if r == '\t' {
			for i := 0; i < 4 && x < d.x+d.w; i++ {
				cellFg, cellBg := d.selectionFgBg(y, x, fg, lineBg)
				scr.Set(x, y, ' ', cellFg, cellBg, "")
				x++
			}
			continue
		}
		cellFg, cellBg := d.selectionFgBg(y, x, fg, lineBg)
		scr.Set(x, y, sanitizeRenderableRune(r), cellFg, cellBg, "")
		x++
	}
	for x < d.x+d.w {
		cellFg, cellBg := d.selectionFgBg(y, x, fg, lineBg)
		scr.Set(x, y, ' ', cellFg, cellBg, "")
		x++
	}
}

func (d *DiffLog) clearRow(scr *screen.Screen, y int, bg string) {
	for x := d.x; x < d.x+d.w; x++ {
		cellFg, cellBg := d.selectionFgBg(y, x, diffFgDefault, bg)
		scr.Set(x, y, ' ', cellFg, cellBg, "")
	}
}
