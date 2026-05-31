package components

import (
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

type Popup struct {
	parent layout.Component
	x, y   int
	w, h   int
	style  layout.Style
	title  string
	query  string
	items  []PopupItem
	active int
	panel  bool
}

type PopupItem struct {
	Title    string
	Detail   string
	Disabled bool
}

func NewPopup() *Popup {
	style := layout.Style{}
	style.SetBackground(30, 27, 38)
	style.SetForeground(232, 229, 241)
	return &Popup{style: style, title: "Command Palette"}
}

func (p *Popup) SetTitle(title string) {
	p.title = title
}

func (p *Popup) SetQuery(query string) {
	p.query = query
}

func (p *Popup) SetItems(items []PopupItem, active int) {
	p.items = items
	p.active = active
}

func (p *Popup) SetPanel(panel bool) {
	p.panel = panel
}

func (p *Popup) IsDirty() bool { return false }
func (p *Popup) MakeDirty()    {}
func (p *Popup) ClearDirty()   {}

func (p *Popup) Layout(x, y, w, h int) {
	p.x = x
	p.y = y
	p.w = w
	p.h = h
}

func (p *Popup) Render(s *screen.Screen) {
	if p.panel {
		p.renderPanel(s)
		return
	}

	overlayBg := ansiBg(8, 7, 10)
	overlayFg := ansiFg(98, 95, 108)
	for y := p.y; y < p.y+p.h; y++ {
		for x := p.x; x < p.x+p.w; x++ {
			s.Set(x, y, ' ', overlayFg, overlayBg, "")
		}
	}

	modalW := clamp(p.w-4, 42, 68)
	modalH := clamp(len(p.items)*2+6, 10, p.h-2)
	mx := p.x + (p.w-modalW)/2
	my := p.y + (p.h-modalH)/2
	bg := p.style.GetBackground()
	fg := p.style.GetForeground()
	muted := ansiFg(159, 153, 176)
	accent := ansiFg(247, 184, 90)

	for y := my; y < my+modalH; y++ {
		for x := mx; x < mx+modalW; x++ {
			ch := ' '
			if y == my || y == my+modalH-1 {
				ch = '─'
			}
			if x == mx || x == mx+modalW-1 {
				ch = '│'
			}
			s.Set(x, y, ch, muted, bg, "")
		}
	}
	s.Set(mx, my, '╭', muted, bg, "")
	s.Set(mx+modalW-1, my, '╮', muted, bg, "")
	s.Set(mx, my+modalH-1, '╰', muted, bg, "")
	s.Set(mx+modalW-1, my+modalH-1, '╯', muted, bg, "")

	drawCenteredText(s, mx+3, my+1, modalW-6, p.title, accent, bg)
	drawSearch(s, mx+3, my+2, modalW-6, p.query, fg, muted, ansiBg(20, 18, 26))
	visibleItems := (modalH - 6) / 2
	start, end := visibleWindow(p.active, len(p.items), visibleItems)
	if len(p.items) > visibleItems && visibleItems > 0 {
		label := rangeLabel(start, end, len(p.items))
		drawText(s, mx+modalW-len(label)-3, my+1, len(label), label, muted, bg)
	}

	row := my + 5
	activeBg := ansiBg(48, 43, 61)
	for i := start; i < end; i++ {
		item := p.items[i]
		itemFg := fg
		if item.Disabled {
			itemFg = muted
		}
		if i == p.active && !item.Disabled {
			drawFill(s, mx+2, row, modalW-4, activeBg)
			drawFill(s, mx+2, row+1, modalW-4, activeBg)
			drawCenteredText(s, mx+2, row, modalW-4, "› "+item.Title+" ‹", accent, activeBg)
			drawCenteredText(s, mx+2, row+1, modalW-4, item.Detail, muted, activeBg)
		} else {
			drawFill(s, mx+2, row, modalW-4, bg)
			drawFill(s, mx+2, row+1, modalW-4, bg)
			drawCenteredText(s, mx+2, row, modalW-4, item.Title, itemFg, bg)
			drawCenteredText(s, mx+2, row+1, modalW-4, item.Detail, muted, bg)
		}
		row += 2
	}
}

func (p *Popup) renderPanel(s *screen.Screen) {
	bg := p.style.GetBackground()
	fg := p.style.GetForeground()
	muted := ansiFg(159, 153, 176)
	accent := ansiFg(247, 184, 90)
	activeBg := ansiBg(48, 43, 61)

	for y := p.y; y < p.y+p.h; y++ {
		for x := p.x; x < p.x+p.w; x++ {
			s.Set(x, y, ' ', fg, bg, "")
		}
	}

	drawCenteredText(s, p.x, p.y, p.w, p.title, accent, bg)
	drawSearch(s, p.x, p.y+1, p.w, p.query, fg, muted, ansiBg(20, 18, 26))
	visibleItems := (p.h - 2) / 3
	start, end := visibleWindow(p.active, len(p.items), visibleItems)
	if len(p.items) > visibleItems && visibleItems > 0 {
		label := rangeLabel(start, end, len(p.items))
		drawText(s, p.x+p.w-len(label), p.y, len(label), label, muted, bg)
	}

	row := p.y + 3
	for i := start; i < end; i++ {
		item := p.items[i]
		if row >= p.y+p.h {
			return
		}
		itemFg := fg
		if item.Disabled {
			itemFg = muted
		}
		if i == p.active && !item.Disabled {
			drawFill(s, p.x, row, p.w, activeBg)
			drawCenteredText(s, p.x, row, p.w, "› "+item.Title+" ‹", accent, activeBg)
			if row+1 < p.y+p.h {
				drawFill(s, p.x, row+1, p.w, activeBg)
				drawCenteredText(s, p.x, row+1, p.w, item.Detail, muted, activeBg)
			}
			if row+2 < p.y+p.h {
				drawFill(s, p.x, row+2, p.w, activeBg)
			}
		} else {
			drawFill(s, p.x, row, p.w, bg)
			drawCenteredText(s, p.x, row, p.w, item.Title, itemFg, bg)
			if row+1 < p.y+p.h {
				drawFill(s, p.x, row+1, p.w, bg)
				drawCenteredText(s, p.x, row+1, p.w, item.Detail, muted, bg)
			}
			if row+2 < p.y+p.h {
				drawFill(s, p.x, row+2, p.w, bg)
			}
		}
		row += 3
	}
}

func (p *Popup) GetStyle() layout.Style { return p.style }

func (p *Popup) SetParent(parent layout.Component) {
	p.parent = parent
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func visibleWindow(active, total, visible int) (int, int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if visible >= total {
		return 0, total
	}
	if active < 0 {
		active = 0
	}
	if active >= total {
		active = total - 1
	}
	start := active - visible/2
	if start < 0 {
		start = 0
	}
	if start+visible > total {
		start = total - visible
	}
	return start, start + visible
}

func rangeLabel(start, end, total int) string {
	return itoa(start+1) + "-" + itoa(end) + "/" + itoa(total)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	digits := [20]byte{}
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

func drawText(s *screen.Screen, x, y, maxW int, text, fg, bg string) {
	for i, r := range text {
		if i >= maxW {
			return
		}
		s.Set(x+i, y, r, fg, bg, "")
	}
}

func drawFill(s *screen.Screen, x, y, w int, bg string) {
	for i := 0; i < w; i++ {
		s.Set(x+i, y, ' ', ansiFg(232, 229, 241), bg, "")
	}
}

func drawCenteredText(s *screen.Screen, x, y, w int, text, fg, bg string) {
	runes := []rune(text)
	textLen := len(runes)
	if textLen > w {
		textLen = w
		runes = runes[:w]
	}
	pad := (w - textLen) / 2
	for i := 0; i < textLen; i++ {
		s.Set(x+pad+i, y, runes[i], fg, bg, "")
	}
}

func drawSearch(s *screen.Screen, x, y, w int, query, fg, muted, bg string) {
	if w <= 0 {
		return
	}
	drawFill(s, x, y, w, bg)
	text := query
	if query == "" {
		text = "Search..."
		fg = muted
	}
	drawCenteredText(s, x, y, w, text, fg, bg)
}

func ansiFg(r, g, b uint8) string {
	style := layout.Style{}
	style.SetForeground(r, g, b)
	return style.GetForeground()
}

func ansiBg(r, g, b uint8) string {
	style := layout.Style{}
	style.SetBackground(r, g, b)
	return style.GetBackground()
}
