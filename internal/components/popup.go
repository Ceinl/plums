package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type Popup struct {
	parent layout.Component
	x, y   int
	w, h   int
	style  layout.Style
	title  string
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
	modalH := clamp(len(p.items)*2+4, 8, p.h-2)
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
	s.Set(mx, my, '┌', muted, bg, "")
	s.Set(mx+modalW-1, my, '┐', muted, bg, "")
	s.Set(mx, my+modalH-1, '└', muted, bg, "")
	s.Set(mx+modalW-1, my+modalH-1, '┘', muted, bg, "")

	drawText(s, mx+3, my+1, modalW-6, p.title, accent, bg)

	row := my + 3
	for i, item := range p.items {
		itemFg := fg
		if item.Disabled {
			itemFg = muted
		}
		if i == p.active && !item.Disabled {
			drawFill(s, mx+2, row, modalW-4, ansiBg(48, 43, 61))
			drawText(s, mx+4, row, modalW-8, "› "+item.Title, accent, ansiBg(48, 43, 61))
			drawText(s, mx+6, row+1, modalW-10, item.Detail, muted, ansiBg(48, 43, 61))
		} else {
			drawText(s, mx+4, row, modalW-8, "  "+item.Title, itemFg, bg)
			drawText(s, mx+6, row+1, modalW-10, item.Detail, muted, bg)
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

	drawText(s, p.x, p.y, p.w, p.title, accent, bg)

	row := p.y + 2
	for i, item := range p.items {
		if row >= p.y+p.h {
			return
		}
		itemFg := fg
		if item.Disabled {
			itemFg = muted
		}
		if i == p.active && !item.Disabled {
			drawFill(s, p.x, row, p.w, activeBg)
			drawText(s, p.x+1, row, p.w-1, "> "+item.Title, accent, activeBg)
			if row+1 < p.y+p.h {
				drawFill(s, p.x, row+1, p.w, activeBg)
				drawText(s, p.x+3, row+1, p.w-3, item.Detail, muted, activeBg)
			}
		} else {
			drawText(s, p.x+1, row, p.w-1, "  "+item.Title, itemFg, bg)
			if row+1 < p.y+p.h {
				drawText(s, p.x+3, row+1, p.w-3, item.Detail, muted, bg)
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
