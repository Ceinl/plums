package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

const (
	infoTabsBg       = "\x1b[48;2;22;20;27m"
	infoTabsHeaderBg = "\x1b[48;2;18;16;23m"
	infoTabsInactive = "\x1b[38;2;92;88;108m"
	infoTabsActive   = "\x1b[38;2;238;234;248m"
)

type InfoTab struct {
	Label  string
	Active bool
}

type InfoTabs struct {
	isDirty bool
	tabs    []InfoTab
	parent  layout.Component
	x, y    int
	w, h    int
	style   layout.Style
}

func NewInfoTabs() *InfoTabs { return &InfoTabs{} }

func (t *InfoTabs) SetTabs(tabs []InfoTab) {
	t.tabs = tabs
	t.isDirty = true
}

func (t *InfoTabs) IsDirty() bool                { return t.isDirty }
func (t *InfoTabs) MakeDirty()                   { t.isDirty = true }
func (t *InfoTabs) ClearDirty()                  { t.isDirty = false }
func (t *InfoTabs) GetStyle() layout.Style       { return t.style }
func (t *InfoTabs) SetParent(p layout.Component) { t.parent = p }
func (t *InfoTabs) SetStyle(st layout.Style)     { t.style = st }

func (t *InfoTabs) Layout(x, y, w, h int) {
	t.x, t.y, t.w, t.h = x, y, w, h
}

func (t *InfoTabs) Render(scr *screen.Screen) {
	if t.w <= 0 || t.h <= 0 {
		return
	}

	bg := t.style.GetBackground()
	if t.parent != nil {
		bg = t.parent.GetStyle().GetBackground()
	}
	if bg == "\x1b[48;2;0;0;0m" {
		bg = infoTabsBg
	}

	for x := 0; x < t.w; x++ {
		scr.Set(t.x+x, t.y, ' ', infoTabsInactive, infoTabsHeaderBg, "")
	}

	cx := t.x + 2
	for i, tab := range t.tabs {
		fg := infoTabsInactive
		if tab.Active {
			fg = infoTabsActive
		}

		if i > 0 {
			for range 3 {
				if cx >= t.x+t.w {
					break
				}
				scr.Set(cx, t.y, ' ', infoTabsInactive, infoTabsHeaderBg, "")
				cx++
			}
		}

		label := tab.Label
		for _, r := range label {
			if cx >= t.x+t.w {
				break
			}
			scr.Set(cx, t.y, r, fg, infoTabsHeaderBg, "")
			cx++
		}
	}
}
