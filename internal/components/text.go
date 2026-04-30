package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type Text struct {
	isDirty bool
	content string
	parent  layout.Component
	x, y    int
	w, h    int
	style   layout.Style
}

func NewText() *Text {
	return &Text{}
}

func (t *Text) IsDirty() bool { return t.isDirty }
func (t *Text) MakeDirty()    { t.isDirty = true }
func (t *Text) ClearDirty()   { t.isDirty = false }

func (t *Text) Layout(x, y, w, h int) {
	t.x = x
	t.y = y
	t.w = w
	t.h = h
}

func (t *Text) Render(s *screen.Screen) {
	bg := t.style.GetBackground()
	fg := t.style.GetForeground()
	decor := t.style.GetDecor()

	if t.parent != nil {
		ps := t.parent.GetStyle()
		bg = ps.GetBackground()
		fg = ps.GetForeground()
		decor = ps.GetDecor()
	}

	cx, cy := t.x, t.y
	for _, r := range t.content {
		if r == '\n' {
			cx = t.x
			cy++
			if cy >= t.y+t.h {
				break
			}
			continue
		}
		if cx >= t.x+t.w {
			cx = t.x
			cy++
			if cy >= t.y+t.h {
				break
			}
		}
		s.Set(cx, cy, r, fg, bg, decor)
		cx++
	}
}

func (t *Text) GetStyle() layout.Style { return t.style }

func (t *Text) SetParent(p layout.Component) { t.parent = p }

func (t *Text) SetContent(c string) {
	if t.content != c {
		t.content = c
		t.isDirty = true
	}
}
