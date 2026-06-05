package components

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

const (
	sessionsMaxHorizontalItems = 8

	sessionsBg        = "\x1b[48;2;25;23;32m"
	sessionsCardBg    = "\x1b[48;2;34;31;43m"
	sessionsActiveBg  = "\x1b[48;2;62;52;78m"
	sessionsFg        = "\x1b[38;2;226;222;235m"
	sessionsMutedFg   = "\x1b[38;2;132;126;149m"
	sessionsAccentFg  = "\x1b[38;2;247;184;90m"
	sessionsCurrentFg = "\x1b[38;2;167;226;185m"
)

type SessionsOrientation int

const (
	SessionsVertical SessionsOrientation = iota
	SessionsHorizontal
)

type SessionMouseAction int

const (
	SessionMouseNone SessionMouseAction = iota
	SessionMouseNew
	SessionMouseSelect
)

type SessionItem struct {
	ID        string
	Title     string
	Directory string
	Updated   int64
	Current   bool
}

type Sessions struct {
	isDirty     bool
	orientation SessionsOrientation
	items       []SessionItem
	collapsed   map[string]bool
	hits        []sessionHit
	parent      layout.Component
	x, y        int
	w, h        int
	style       layout.Style
	scroll      int
}

type sessionHit struct {
	x, y, w, h int
	action     SessionMouseAction
	id         string
	project    string
}

type sessionRow struct {
	kind    sessionRowKind
	item    SessionItem
	project string
}

type sessionRowKind int

const (
	sessionRowNew sessionRowKind = iota
	sessionRowGap
	sessionRowProject
	sessionRowItem
)

func NewSessions(orientation SessionsOrientation) *Sessions {
	style := layout.Style{}
	style.SetBackground(25, 23, 32)
	style.SetForeground(226, 222, 235)
	return &Sessions{orientation: orientation, collapsed: map[string]bool{}, style: style}
}

func (s *Sessions) SetOrientation(orientation SessionsOrientation) {
	if s.orientation != orientation {
		s.orientation = orientation
		s.scroll = 0
		s.isDirty = true
	}
}

func (s *Sessions) SetItems(items []SessionItem) {
	s.items = append([]SessionItem(nil), items...)
	s.clampScroll()
	s.isDirty = true
}

func (s *Sessions) IsDirty() bool                { return s.isDirty }
func (s *Sessions) MakeDirty()                   { s.isDirty = true }
func (s *Sessions) ClearDirty()                  { s.isDirty = false }
func (s *Sessions) GetStyle() layout.Style       { return s.style }
func (s *Sessions) SetStyle(st layout.Style)     { s.style = st }
func (s *Sessions) SetParent(p layout.Component) { s.parent = p }

func (s *Sessions) Layout(x, y, w, h int) {
	s.x, s.y, s.w, s.h = x, y, w, h
	s.clampScroll()
}

func (s *Sessions) Render(scr *screen.Screen) {
	if s.w <= 0 || s.h <= 0 {
		return
	}
	s.hits = nil
	if s.orientation == SessionsHorizontal {
		s.renderHorizontal(scr)
	} else {
		s.renderVertical(scr)
	}
}

func (s *Sessions) MouseDown(x, y int) (SessionMouseAction, string, bool) {
	for _, hit := range s.hits {
		if x < hit.x || x >= hit.x+hit.w || y < hit.y || y >= hit.y+hit.h {
			continue
		}
		if hit.project != "" {
			s.collapsed[hit.project] = !s.collapsed[hit.project]
			s.clampScroll()
			s.isDirty = true
			return SessionMouseNone, "", false
		}
		return hit.action, hit.id, hit.action != SessionMouseNone
	}
	return SessionMouseNone, "", false
}

func (s *Sessions) Scroll(delta int) bool {
	before := s.scroll
	s.scroll -= delta
	s.clampScroll()
	if s.scroll != before {
		s.isDirty = true
	}
	return s.scroll != before
}

func (s *Sessions) Contains(x, y int) bool {
	return x >= s.x && x < s.x+s.w && y >= s.y && y < s.y+s.h
}

func (s *Sessions) renderVertical(scr *screen.Screen) {
	bg := s.background()
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			scr.Set(s.x+x, s.y+y, ' ', sessionsFg, bg, "")
		}
	}

	rows := s.verticalRows()
	visible := rows
	if s.scroll < len(rows) {
		visible = rows[s.scroll:]
	} else {
		visible = nil
	}
	for rowY, row := range visible {
		if rowY >= s.h {
			break
		}
		absY := s.y + rowY
		switch row.kind {
		case sessionRowNew:
			s.drawCard(scr, s.x, absY, s.w, "+ New session", false, true)
			s.hits = append(s.hits, sessionHit{x: s.x, y: absY, w: s.w, h: 1, action: SessionMouseNew})
		case sessionRowProject:
			label := projectLabel(row.project)
			prefix := "▾ "
			if s.collapsed[row.project] {
				prefix = "▸ "
			}
			drawText(scr, s.x, absY, s.w, prefix+label, sessionsMutedFg, bg)
			s.hits = append(s.hits, sessionHit{x: s.x, y: absY, w: s.w, h: 1, project: row.project})
		case sessionRowItem:
			title := sessionTitle(row.item)
			s.drawCard(scr, s.x, absY, s.w, title, row.item.Current, false)
			s.hits = append(s.hits, sessionHit{x: s.x, y: absY, w: s.w, h: 1, action: SessionMouseSelect, id: row.item.ID})
		}
	}
}

func (s *Sessions) renderHorizontal(scr *screen.Screen) {
	bg := s.background()
	for y := 0; y < s.h; y++ {
		for x := 0; x < s.w; x++ {
			scr.Set(s.x+x, s.y+y, ' ', sessionsFg, bg, "")
		}
	}
	if s.h <= 0 {
		return
	}

	y := s.y + s.h/2
	plusW := minInt(3, s.w)
	contentW := s.w - plusW
	items := s.horizontalItems()
	cx := s.x - s.scroll
	for _, item := range items {
		label := " " + sessionTitle(item) + " "
		if item.Current {
			label = " " + sessionTitle(item) + "* "
		}
		tabW := clampInt(len([]rune(label)), 6, 18)
		if cx+tabW > s.x && cx < s.x+contentW {
			s.drawTab(scr, cx, y, tabW, label, item.Current)
			hitX := maxInt(cx, s.x)
			hitW := minInt(cx+tabW, s.x+contentW) - hitX
			if hitW > 0 {
				s.hits = append(s.hits, sessionHit{x: hitX, y: y, w: hitW, h: 1, action: SessionMouseSelect, id: item.ID})
			}
		}
		cx += tabW + 1
	}
	if plusW > 0 {
		px := s.x + s.w - plusW
		drawFill(scr, px, y, plusW, sessionsCardBg)
		drawCenteredText(scr, px, y, plusW, "+", sessionsAccentFg, sessionsCardBg)
		s.hits = append(s.hits, sessionHit{x: px, y: y, w: plusW, h: 1, action: SessionMouseNew})
	}
}

func (s *Sessions) verticalRows() []sessionRow {
	rows := []sessionRow{{kind: sessionRowNew}, {kind: sessionRowGap}}
	items := append([]SessionItem(nil), s.items...)
	hasProjects := false
	for _, item := range items {
		if item.Directory != "" {
			hasProjects = true
			break
		}
	}
	if !hasProjects {
		for _, item := range items {
			rows = append(rows, sessionRow{kind: sessionRowItem, item: item}, sessionRow{kind: sessionRowGap})
		}
		if len(rows) > 0 && rows[len(rows)-1].kind == sessionRowGap {
			rows = rows[:len(rows)-1]
		}
		return rows
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Directory != items[j].Directory {
			return items[i].Directory < items[j].Directory
		}
		return items[i].Updated > items[j].Updated
	})

	lastProject := ""
	for _, item := range items {
		project := item.Directory
		if project == "" {
			project = "No project"
		}
		if project != lastProject {
			if lastProject != "" && len(rows) > 0 && rows[len(rows)-1].kind != sessionRowGap {
				rows = append(rows, sessionRow{kind: sessionRowGap})
			}
			rows = append(rows, sessionRow{kind: sessionRowProject, project: project})
			lastProject = project
		}
		if !s.collapsed[project] {
			rows = append(rows, sessionRow{kind: sessionRowItem, item: item, project: project}, sessionRow{kind: sessionRowGap})
		}
	}
	if len(rows) > 0 && rows[len(rows)-1].kind == sessionRowGap {
		rows = rows[:len(rows)-1]
	}
	return rows
}

func (s *Sessions) horizontalItems() []SessionItem {
	items := append([]SessionItem(nil), s.items...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Updated != items[j].Updated {
			return items[i].Updated > items[j].Updated
		}
		return false
	})
	if len(items) > sessionsMaxHorizontalItems {
		items = items[:sessionsMaxHorizontalItems]
	}
	return items
}

func (s *Sessions) clampScroll() {
	max := 0
	if s.orientation == SessionsVertical {
		max = len(s.verticalRows()) - s.h
	} else {
		width := 0
		for _, item := range s.horizontalItems() {
			labelW := clampInt(len([]rune(" "+sessionTitle(item)+" ")), 6, 18)
			width += labelW + 1
		}
		contentW := s.w - minInt(3, s.w)
		max = width - contentW
	}
	if max < 0 {
		max = 0
	}
	if s.scroll > max {
		s.scroll = max
	}
	if s.scroll < 0 {
		s.scroll = 0
	}
}

func (s *Sessions) drawCard(scr *screen.Screen, x, y, w int, text string, current, accent bool) {
	if w <= 0 {
		return
	}
	bg := sessionsCardBg
	fg := sessionsFg
	if current {
		bg = sessionsActiveBg
		fg = sessionsCurrentFg
	} else if accent {
		fg = sessionsAccentFg
	}
	drawFill(scr, x, y, w, bg)
	drawText(scr, x+1, y, maxInt(w-2, 0), text, fg, bg)
}

func (s *Sessions) drawTab(scr *screen.Screen, x, y, w int, text string, current bool) {
	if w <= 0 {
		return
	}
	bg := sessionsCardBg
	fg := sessionsFg
	if current {
		bg = sessionsActiveBg
		fg = sessionsCurrentFg
	}
	drawFill(scr, maxInt(x, s.x), y, minInt(x+w, s.x+s.w)-maxInt(x, s.x), bg)
	drawText(scr, x, y, w, text, fg, bg)
}

func (s *Sessions) background() string {
	bg := s.style.GetBackground()
	if s.parent != nil {
		bg = s.parent.GetStyle().GetBackground()
	}
	if bg == "\x1b[48;2;0;0;0m" {
		bg = sessionsBg
	}
	return bg
}

func sessionTitle(item SessionItem) string {
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = item.ID
	}
	if title == "" {
		return "untitled"
	}
	return title
}

func projectLabel(project string) string {
	if project == "No project" {
		return project
	}
	base := filepath.Base(project)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return project
	}
	return base
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
