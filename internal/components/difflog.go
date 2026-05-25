package components

import (
	"strings"

	"plums/internal/layout"
	"plums/internal/screen"
)

const (
	diffFgDefault = "\x1b[38;2;198;195;210m"
	diffFgFile    = "\x1b[1m\x1b[38;2;165;205;255m"
	diffFgHunk    = "\x1b[38;2;180;145;245m"
	diffFgAdd     = "\x1b[38;2;125;210;150m"
	diffFgRemove  = "\x1b[38;2;240;130;130m"
	diffFgMeta    = "\x1b[38;2;92;88;108m"
	diffBgAdd     = "\x1b[48;2;20;36;28m"
	diffBgRemove  = "\x1b[48;2;42;24;30m"
)

type DiffLog struct {
	isDirty      bool
	content      string
	scrollOffset int
	parent       layout.Component
	x, y         int
	w, h         int
	style        layout.Style
}

func NewDiffLog() *DiffLog { return &DiffLog{} }

func (d *DiffLog) SetContent(content string) {
	d.content = content
	d.isDirty = true
}

func (d *DiffLog) SetScrollOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	d.scrollOffset = offset
	d.isDirty = true
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

func (d *DiffLog) Render(scr *screen.Screen) {
	if d.w <= 0 || d.h <= 0 {
		return
	}

	bg := d.style.GetBackground()
	if d.parent != nil {
		bg = d.parent.GetStyle().GetBackground()
	}

	lines := d.lines()
	start := 0
	if len(lines) > d.h {
		maxStart := len(lines) - d.h
		start = maxStart - d.scrollOffset
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
	if strings.TrimSpace(d.content) == "" {
		return []string{"No git diff."}
	}

	var out []string
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
	return out
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
		scr.Set(x, y, ' ', fg, lineBg, "")
		x++
	}
	for _, r := range line {
		if x >= d.x+d.w {
			break
		}
		if r == '\t' {
			for i := 0; i < 4 && x < d.x+d.w; i++ {
				scr.Set(x, y, ' ', fg, lineBg, "")
				x++
			}
			continue
		}
		scr.Set(x, y, sanitizeRenderableRune(r), fg, lineBg, "")
		x++
	}
	for x < d.x+d.w {
		scr.Set(x, y, ' ', fg, lineBg, "")
		x++
	}
}

func (d *DiffLog) clearRow(scr *screen.Screen, y int, bg string) {
	for x := d.x; x < d.x+d.w; x++ {
		scr.Set(x, y, ' ', diffFgDefault, bg, "")
	}
}
