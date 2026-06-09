package app

import (
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

func (s *State) OutputScroll() int {
	return s.outputScroll
}

func (s *State) ScrollOutput(delta int) bool {
	before := s.outputScroll
	s.outputScroll += delta
	if s.outputScroll < 0 {
		s.outputScroll = 0
	}
	return s.outputScroll != before
}

func (s *State) ScrollOutputVisible(delta int) bool {
	before := s.outputScroll
	s.ScrollOutput(delta)
	if s.outputMaxSet {
		s.ClampOutputScroll(s.outputMax)
	}
	return s.outputScroll != before
}

func (s *State) ScrollAt(x, y, delta int) bool {
	if s.sessions != nil && s.sessions.Contains(x, y) {
		return s.sessions.Scroll(delta)
	}
	if s.sessionsHorizontal != nil && s.sessionsHorizontal.Contains(x, y) {
		return s.sessionsHorizontal.Scroll(delta)
	}
	if s.isEditorPoint(x, y) {
		return s.Editor.Scroll(delta)
	}
	return s.ScrollOutputVisible(delta)
}

func (s *State) isEditorPoint(x, y int) bool {
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return false
	}

	switch s.EffectiveLayout() {
	case LayoutChat:
		return y >= s.height-3
	case LayoutFullscreen:
		return s.FullscreenShowsEditor()
	case LayoutSplit:
		if s.width >= MinSplitLayoutWidth {
			leftW := s.SplitLeftWidth()
			return x < leftW && !s.PopupOpen
		}
		outputH := int(float64(s.height) * 0.5)
		return y > outputH
	default:
		return false
	}
}

func (s *State) ScrollOutputPage(direction int) bool {
	page := s.height - 4
	if page < 1 {
		page = 1
	}
	return s.ScrollOutputVisible(direction * page)
}

func (s *State) ScrollOutputBottom() bool {
	if s.outputScroll == 0 {
		return false
	}
	s.outputScroll = 0
	return true
}

func (s *State) ClampOutputScroll(maxOffset int) {
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.outputScroll > maxOffset {
		s.outputScroll = maxOffset
	}
	if s.outputScroll < 0 {
		s.outputScroll = 0
	}
}

func (s *State) SetOutputMaxScroll(maxOffset int) {
	s.outputMax = maxOffset
	s.outputMaxSet = true
	s.ClampOutputScroll(maxOffset)
}

func (s *State) invalidateOutputMax() {
	// Keep the last rendered max as a provisional clamp until the next render
	// computes the exact value. This prevents boundary scroll input from moving
	// past the visible range and then snapping back during render.
}

func (s *State) SessionMouseDown(x, y int) bool {
	sessions := s.Sessions()
	if s.sessionsHorizontal != nil && s.sessionsHorizontal.Contains(x, y) {
		sessions = s.sessionsHorizontal
	}
	action, id, ok := sessions.MouseDown(x, y)
	if !ok {
		return false
	}
	s.PopupOpen = false
	s.PaletteQuery = ""
	s.PaletteView = PaletteViewSessions
	s.PendingAction = PaletteActionNone
	s.PaletteIndex = 0
	switch action {
	case components.SessionMouseNew:
		s.PendingAction = PaletteActionNewSession
	case components.SessionMouseSelect:
		if id == "" || id == s.SessionID {
			return true
		}
		for i, item := range s.visibleSessionItems() {
			if item.ID == id {
				s.PaletteIndex = i
				s.PendingAction = PaletteActionSelectSession
				break
			}
		}
	}
	return true
}

func (s *State) OutputMouseDown(x, y int) bool {
	s.outputMouseSelecting = false
	if s.InfoView == InfoViewGitDiff {
		s.outputMouseSelecting = s.DiffLog().MouseDown(x, y)
		return s.outputMouseSelecting
	}
	s.outputMouseSelecting = s.ChatLog().MouseDown(x, y)
	return s.outputMouseSelecting
}

func (s *State) OutputMouseDrag(x, y int) bool {
	if !s.outputMouseSelecting {
		return false
	}
	if s.InfoView == InfoViewGitDiff {
		s.DiffLog().MouseDrag(x, y)
	} else {
		s.ChatLog().MouseDrag(x, y)
	}
	return true
}

func (s *State) OutputMouseUp(x, y int) string {
	if !s.outputMouseSelecting {
		return ""
	}
	s.outputMouseSelecting = false
	if s.InfoView == InfoViewGitDiff {
		return s.DiffLog().MouseUp(x, y)
	}
	return s.ChatLog().MouseUp(x, y)
}
