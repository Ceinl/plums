package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type Div struct {
	isDirty bool

	W, H layout.Unit

	justifyContent layout.JustifyContent
	alignItems     layout.AlignItems

	children []layout.Component

	parent layout.Component

	cx, cy int
	cw, ch int

	padding layout.Padding

	style layout.Style
}

func (d *Div) GetStyle() layout.Style {
	return d.style
}

func (d *Div) SetStyle(style layout.Style) {
	d.style = style
}

func NewDiv() *Div {
	return &Div{}
}

func (d *Div) IsDirty() bool {
	return d.isDirty
}

func (d *Div) MakeDirty() {
	d.isDirty = true
}

func (d *Div) ClearDirty() {
	d.isDirty = false
}

func (d *Div) Layout(x, y, w, h int) {
	d.cx = x
	d.cy = y
	d.cw = w
	d.ch = h

	pl := d.padding.Left.Resolve(w)
	pr := d.padding.Right.Resolve(w)
	pt := d.padding.Top.Resolve(h)
	pb := d.padding.Bottom.Resolve(h)

	innerW := w - pl - pr
	innerH := h - pt - pb
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}

	type childSize struct {
		child  layout.Component
		cw, ch int
	}
	var sizes []childSize
	totalGrow := 0
	fixedH := 0

	for _, child := range d.children {
		if cd, ok := child.(*Div); ok {
			var cw, ch int
			switch cd.W.Type {
			case layout.UnitPx:
				cw = int(cd.W.Value)
			case layout.UnitPersent:
				cw = int(float64(innerW) * cd.W.Value / 100.0)
			case layout.UnitGrow:
				cw = innerW
			}
			if cw > innerW {
				cw = innerW
			}
			if cw < 0 {
				cw = 0
			}

			switch cd.H.Type {
			case layout.UnitPx:
				ch = int(cd.H.Value)
			case layout.UnitPersent:
				ch = int(float64(innerH) * cd.H.Value / 100.0)
			case layout.UnitGrow:
				totalGrow++
			}
			if ch < 0 {
				ch = 0
			}

			if cd.H.Type != layout.UnitGrow {
				fixedH += ch
			}
			sizes = append(sizes, childSize{child, cw, ch})
		} else {
			totalGrow++
			sizes = append(sizes, childSize{child, innerW, 0})
		}
	}

	growSize := 0
	if totalGrow > 0 {
		rem := innerH - fixedH
		if rem > 0 {
			growSize = rem / totalGrow
		}
	}

	for i := range sizes {
		if cd, ok := sizes[i].child.(*Div); ok && cd.H.Type == layout.UnitGrow {
			sizes[i].ch = growSize
		} else if _, ok := sizes[i].child.(*Div); !ok {
			sizes[i].ch = growSize
		}
	}

	rem := innerH - fixedH - (growSize * totalGrow)
	for i := range sizes {
		if rem <= 0 {
			break
		}
		if cd, ok := sizes[i].child.(*Div); ok && cd.H.Type == layout.UnitGrow {
			sizes[i].ch++
			rem--
		} else if _, ok := sizes[i].child.(*Div); !ok {
			sizes[i].ch++
			rem--
		}
	}

	cy := y + pt
	for _, sc := range sizes {
		cx := x + pl
		cw := sc.cw
		ch := sc.ch

		switch d.alignItems {
		case layout.ACenter:
			cx += (innerW - cw) / 2
		case layout.ARight:
			cx += innerW - cw
		}

		sc.child.Layout(cx, cy, cw, ch)
		cy += ch
	}
}

func (d *Div) SetSize(w, h layout.Unit) {
	d.W = w
	d.H = h
}

func (d *Div) SetPadding(p layout.Padding) {
	d.padding = p
}

func (d *Div) Render(s *screen.Screen) {
	bg := d.style.GetBackground()
	fg := d.style.GetForeground()
	decor := d.style.GetDecor()

	for y := d.cy; y < d.cy+d.ch; y++ {
		for x := d.cx; x < d.cx+d.cw; x++ {
			s.Set(x, y, ' ', fg, bg, decor)
		}
	}
	for _, child := range d.children {
		child.Render(s)
	}
}

func (d *Div) JustifyContent(jc layout.JustifyContent) {
	d.justifyContent = jc
}

func (d *Div) AlignItems(ai layout.AlignItems) {
	d.alignItems = ai
}

func (d *Div) SetParent(parent layout.Component) {
	d.parent = parent
}

func (d *Div) AppendChild(children layout.Component) {
	children.SetParent(d)
	d.children = append(d.children, children)
}
