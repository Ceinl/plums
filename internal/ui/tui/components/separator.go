package components

import (
	"time"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

var (
	sepFgDefault  string
	sepFgReady    string
	sepFgStarting string
	sepFgThinking string
	sepBgDefault  string
)

func init() { registerColorRefresher(refreshSeparatorColors) }

func refreshSeparatorColors() {
	sepFgDefault = theme.TextFaint.Fg()
	sepFgReady = theme.StatusReady.Fg()
	sepFgStarting = theme.StatusStarting.Fg()
	sepFgThinking = theme.StatusThinking.Fg()
	sepBgDefault = theme.BgHighlight.Bg()
}

var separatorSpinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

type Separator struct {
	isDirty        bool
	serverStarting bool
	serverReady    bool
	aiThinking     bool
	parent         layout.Component
	x, y, w, h     int
	style          layout.Style
}

func NewSeparator() *Separator {
	return &Separator{}
}

func (s *Separator) IsDirty() bool                { return s.isDirty }
func (s *Separator) MakeDirty()                   { s.isDirty = true }
func (s *Separator) ClearDirty()                  { s.isDirty = false }
func (s *Separator) GetStyle() layout.Style       { return s.style }
func (s *Separator) SetParent(p layout.Component) { s.parent = p }
func (s *Separator) SetStyle(st layout.Style)     { s.style = st }

func (s *Separator) SetStatus(serverStarting, serverReady, aiThinking bool) {
	s.serverStarting = serverStarting
	s.serverReady = serverReady
	s.aiThinking = aiThinking
	s.isDirty = true
}

func (s *Separator) Layout(x, y, w, h int) {
	s.x, s.y, s.w, s.h = x, y, w, h
}

func (s *Separator) Render(scr *screen.Screen) {
	bg := sepBgDefault
	ch := '─'
	if s.w == 1 && s.h > 1 {
		ch = '│'
	}

	for y := s.y; y < s.y+s.h; y++ {
		for x := s.x; x < s.x+s.w; x++ {
			scr.Set(x, y, ch, sepFgDefault, bg, "")
		}
	}

	if s.w == 0 || s.h == 0 || s.serverStarting {
		if s.w > 0 && s.h > 0 && s.serverStarting {
			idx := (time.Now().UnixMilli() / 80) % int64(len(separatorSpinnerFrames))
			frame := separatorSpinnerFrames[int(idx)]
			scr.Set(s.x, s.y, frame, sepFgStarting, bg, "")
		}
		return
	}

	if s.aiThinking {
		idx := (time.Now().UnixMilli() / 80) % int64(len(separatorSpinnerFrames))
		frame := separatorSpinnerFrames[int(idx)]
		scr.Set(s.x, s.y, frame, sepFgThinking, bg, "")
		return
	}

	if s.serverReady {
		scr.Set(s.x, s.y, '•', sepFgReady, bg, "")
	}
}
