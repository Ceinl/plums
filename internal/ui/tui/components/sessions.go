package components

import (
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

const (
	sessionsBg       = "\x1b[48;2;22;20;27m"
	sessionsTabBg    = "\x1b[48;2;32;30;40m"
	sessionsActiveBg = "\x1b[48;2;47;43;59m"
	sessionsFg       = "\x1b[38;2;159;153;176m"
	sessionsActiveFg = "\x1b[38;2;238;234;248m"
	sessionsAccentFg = "\x1b[38;2;247;184;90m"
)

type SessionsOrientation int

const (
	SessionsVertical SessionsOrientation = iota
	SessionsHorizontal
)

type SessionItem struct {
	ID      string
	Title   string
	Current bool
}

type SessionMouseAction int

const (
	SessionMouseNone SessionMouseAction = iota
	SessionMouseNew
	SessionMouseSelect
)

type sessionHitBox struct {
	x, y, w, h int
	id         string
	new        bool
}

type Sessions struct {
	isDirty     bool
	items       []SessionItem
	orientation SessionsOrientation
	parent      layout.Component
	x, y        int
	w, h        int
	style       layout.Style
	hits        []sessionHitBox
}

func NewSessions(orientation SessionsOrientation) *Sessions {
	return &Sessions{orientation: orientation}
}

func (s *Sessions) SetItems(items []SessionItem) {
	s.items = items
	s.isDirty = true
}

func (s *Sessions) SetOrientation(orientation SessionsOrientation) {
	s.orientation = orientation
	s.isDirty = true
}

func (s *Sessions) IsDirty() bool                { return s.isDirty }
func (s *Sessions) MakeDirty()                   { s.isDirty = true }
func (s *Sessions) ClearDirty()                  { s.isDirty = false }
func (s *Sessions) GetStyle() layout.Style       { return s.style }
func (s *Sessions) SetParent(p layout.Component) { s.parent = p }
func (s *Sessions) SetStyle(st layout.Style)     { s.style = st }

func (s *Sessions) Layout(x, y, w, h int) {
	s.x, s.y, s.w, s.h = x, y, w, h
}

func (s *Sessions) MouseDown(x, y int) (SessionMouseAction, string, bool) {
	for _, hit := range s.hits {
		if x < hit.x || x >= hit.x+hit.w || y < hit.y || y >= hit.y+hit.h {
			continue
		}
		if hit.new {
			return SessionMouseNew, "", true
		}
		return SessionMouseSelect, hit.id, true
	}
	return SessionMouseNone, "", false
}

func (s *Sessions) Render(scr *screen.Screen) {
	if s.w <= 0 || s.h <= 0 {
		return
	}

	bg := s.style.GetBackground()
	if s.parent != nil {
		bg = s.parent.GetStyle().GetBackground()
	}
	if bg == "\x1b[48;2;0;0;0m" {
		bg = sessionsBg
	}

	s.hits = s.hits[:0]
	for yy := 0; yy < s.h; yy++ {
		for xx := 0; xx < s.w; xx++ {
			scr.Set(s.x+xx, s.y+yy, ' ', sessionsFg, bg, "")
		}
	}

	if s.orientation == SessionsHorizontal {
		s.renderHorizontal(scr, bg)
		return
	}
	s.renderVertical(scr, bg)
}

func (s *Sessions) renderVertical(scr *screen.Screen, bg string) {
	button := "+ New session"
	s.drawText(scr, s.x+1, s.y, s.w-2, button, sessionsAccentFg, bg)
	s.hits = append(s.hits, sessionHitBox{x: s.x, y: s.y, w: s.w, h: 1, new: true})

	row := s.y + 2
	for _, item := range s.items {
		if row >= s.y+s.h {
			return
		}
		fg := sessionsFg
		if item.Current {
			fg = sessionsActiveFg
		}
		label := sessionLabel(item)
		if item.Current {
			label = "> " + label
		} else {
			label = "  " + label
		}
		s.drawText(scr, s.x+1, row, s.w-2, label, fg, bg)
		s.hits = append(s.hits, sessionHitBox{x: s.x, y: row, w: s.w, h: 1, id: item.ID})
		row++
	}
}

func (s *Sessions) renderHorizontal(scr *screen.Screen, bg string) {
	plus := " + "
	plusX := s.x + s.w - len(plus)
	if plusX < s.x {
		plusX = s.x
	}
	s.drawBlock(scr, plusX, s.y, len(plus), 1, sessionsAccentFg, sessionsTabBg)
	s.drawText(scr, plusX, s.y, len(plus), plus, sessionsAccentFg, sessionsTabBg)
	s.hits = append(s.hits, sessionHitBox{x: plusX, y: s.y, w: len(plus), h: 1, new: true})

	cx := s.x
	maxX := plusX - 1
	for _, item := range s.items {
		if cx >= maxX {
			return
		}
		label := " " + sessionLabel(item) + " "
		width := len([]rune(label))
		if width > 24 {
			label = truncateWithEllipsis(label, 24)
			width = len([]rune(label))
		}
		if cx+width > maxX {
			width = maxX - cx
			label = truncateRunesLocal(label, width)
		}
		fg := sessionsFg
		tabBg := sessionsTabBg
		if item.Current {
			fg = sessionsActiveFg
			tabBg = sessionsActiveBg
		}
		s.drawBlock(scr, cx, s.y, width, 1, fg, tabBg)
		s.drawText(scr, cx, s.y, width, label, fg, tabBg)
		s.hits = append(s.hits, sessionHitBox{x: cx, y: s.y, w: width, h: 1, id: item.ID})
		cx += width + 1
	}
}

func (s *Sessions) drawBlock(scr *screen.Screen, x, y, w, h int, fg, bg string) {
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			scr.Set(x+xx, y+yy, ' ', fg, bg, "")
		}
	}
}

func (s *Sessions) drawText(scr *screen.Screen, x, y, maxW int, text, fg, bg string) {
	if maxW <= 0 {
		return
	}
	col := 0
	for _, r := range text {
		if col >= maxW {
			return
		}
		scr.Set(x+col, y, r, fg, bg, "")
		col++
	}
}

func sessionLabel(item SessionItem) string {
	label := strings.TrimSpace(item.Title)
	if label == "" {
		label = strings.TrimSpace(item.ID)
	}
	if label == "" {
		return "untitled"
	}
	return label
}

func truncateWithEllipsis(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	return string(runes[:width-3]) + "..."
}

func truncateRunesLocal(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}
