package components

import (
	"strings"
	"time"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

const (
	statusFgDefault  = "\x1b[38;2;100;98;112m"
	statusFgReady    = "\x1b[38;2;120;130;145m"
	statusFgStarting = "\x1b[38;2;245;190;70m"
	statusFgThinking = "\x1b[38;2;100;190;255m"
)

type StatusBar struct {
	isDirty        bool
	serverStarting bool
	serverReady    bool
	aiThinking     bool
	mode           string
	providerID     string
	modelID        string
	sessionTitle   string
	showSession    bool
	parent         layout.Component
	x, y, w, h     int
	style          layout.Style
}

func NewStatusBar() *StatusBar {
	return &StatusBar{showSession: true}
}

func (s *StatusBar) IsDirty() bool                { return s.isDirty }
func (s *StatusBar) MakeDirty()                   { s.isDirty = true }
func (s *StatusBar) ClearDirty()                  { s.isDirty = false }
func (s *StatusBar) GetStyle() layout.Style       { return s.style }
func (s *StatusBar) SetParent(p layout.Component) { s.parent = p }
func (s *StatusBar) SetStyle(st layout.Style)     { s.style = st }

func (s *StatusBar) SetStatus(serverStarting, serverReady, aiThinking bool) {
	s.serverStarting = serverStarting
	s.serverReady = serverReady
	s.aiThinking = aiThinking
	s.isDirty = true
}

func (s *StatusBar) SetModel(providerID, modelID string) {
	s.providerID = providerID
	s.modelID = modelID
	s.isDirty = true
}

func (s *StatusBar) SetMode(mode string) {
	s.mode = mode
	s.isDirty = true
}

func (s *StatusBar) SetSession(title string) {
	s.sessionTitle = title
	s.isDirty = true
}

func (s *StatusBar) SetShowSession(v bool) {
	s.showSession = v
	s.isDirty = true
}

func (s *StatusBar) Layout(x, y, w, h int) {
	s.x, s.y, s.w, s.h = x, y, w, h
}

func (s *StatusBar) Render(scr *screen.Screen) {
	if s.w == 0 || s.h == 0 {
		return
	}

	bg := s.style.GetBackground()
	if s.parent != nil {
		bg = s.parent.GetStyle().GetBackground()
	}

	fg := statusFgDefault
	icon := '•'
	label := "offline"
	switch {
	case s.serverStarting:
		fg = statusFgStarting
		icon = separatorSpinnerFrames[(time.Now().UnixMilli()/80)%int64(len(separatorSpinnerFrames))]
		label = "starting"
	case s.aiThinking:
		fg = statusFgThinking
		icon = separatorSpinnerFrames[(time.Now().UnixMilli()/80)%int64(len(separatorSpinnerFrames))]
		label = "thinking"
	case s.serverReady:
		fg = statusFgReady
		label = "ready"
	}

	model := "model pending"
	if s.providerID != "" && s.modelID != "" {
		model = s.providerID + "/" + s.modelID
	} else if s.modelID != "" {
		model = s.modelID
	}

	var parts []string
	parts = append(parts, string(icon)+" "+label)
	if s.showSession {
		session := s.sessionTitle
		if session == "" {
			session = "untitled session"
		}
		parts = append(parts, session)
	}
	mode := s.mode
	if mode == "" {
		mode = "build"
	}
	parts = append(parts, mode, model)
	content := strings.TrimSpace(strings.Join(parts, "  "))
	content = truncateRunes(content, s.w)
	for i, r := range content {
		scr.Set(s.x+i, s.y, r, fg, bg, "")
	}
}

func truncateRunes(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	return string(runes[:width])
}
