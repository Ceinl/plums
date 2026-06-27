package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

// activeOutputScrollable returns the on-screen public component that owns a
// scrollable body (e.g. chat_output), or nil when the active output pane is still
// a legacy State-driven pane (diff log).
func (s *State) activeOutputScrollable() capabilities.Scrollable {
	for i := len(s.publicComponents) - 1; i >= 0; i-- {
		if sc, ok := s.publicComponents[i].component.(capabilities.Scrollable); ok {
			return sc
		}
	}
	return nil
}

// scrollableAt returns the public scrollable component under (x, y), so wheel
// scroll targets the pane actually beneath the cursor — independent of any other
// scrollable panes in the layout.
func (s *State) scrollableAt(x, y int) capabilities.Scrollable {
	for i := len(s.publicComponents) - 1; i >= 0; i-- {
		adapter := s.publicComponents[i]
		if sc, ok := adapter.component.(capabilities.Scrollable); ok && adapter.contains(x, y) {
			return sc
		}
	}
	return nil
}

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
	if sc := s.activeOutputScrollable(); sc != nil {
		return sc.Scroll(delta)
	}
	return s.scrollOutputOffset(delta)
}

func (s *State) scrollOutputOffset(delta int) bool {
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
	if sc := s.scrollableAt(x, y); sc != nil {
		return sc.Scroll(delta)
	}
	return s.ScrollOutputVisible(delta)
}

func (s *State) isEditorPoint(x, y int) bool {
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return false
	}

	switch s.EffectiveLayout() {
	case LayoutSplit:
		if s.width >= MinSplitLayoutWidth {
			leftW := s.SplitLeftWidth()
			return x < leftW && !s.PopupOpen
		}
		outputH := int(float64(s.height) * 0.5)
		return y > outputH
	default:
		// Chat, zen and other simple bottom-editor layouts.
		return y >= s.height-3
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
	if sc := s.activeOutputScrollable(); sc != nil {
		return sc.ScrollToBottom()
	}
	return s.scrollOutputOffsetBottom()
}

func (s *State) scrollOutputOffsetBottom() bool {
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

func (s *State) SessionMouseDown(ctx capabilities.Ctx, x, y int) bool {
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
	s.PaletteIndex = 0
	switch action {
	case components.SessionMouseNew:
		if ctx != nil {
			ctx.NewSession()
		}
	case components.SessionMouseSelect:
		if id == "" || id == s.SessionID {
			return true
		}
		if ctx != nil {
			ctx.OpenSession(id)
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
