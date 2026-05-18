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
	items  []PopupItem
	active int
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
	return &Popup{style: style}
}

func (p *Popup) SetItems(items []PopupItem, active int) {
	p.items = items
	p.active = active
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
	overlayBg := ansiBg(8, 7, 10)
	overlayFg := ansiFg(98, 95, 108)
	for y := p.y; y < p.y+p.h; y++ {
		for x := p.x; x < p.x+p.w; x++ {
			s.Set(x, y, ' ', overlayFg, overlayBg, "")
		}
	}

	modalW := clamp(p.w-4, 42, 68)
	modalH := clamp(len(p.items)*2+5, 9, p.h-2)
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

	drawText(s, mx+3, my+1, modalW-6, "Command Palette", accent, bg)
	drawText(s, mx+3, my+2, modalW-6, "Ctrl+P open  Enter select  Esc close", muted, bg)

	row := my + 4
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
