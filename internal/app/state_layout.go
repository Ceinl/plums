package app

import "strings"

// LayoutType is the identifier of a layout — the same string used as its key in
// the render config's "layouts" map. New layouts are added purely in JSON (a
// layouts entry plus a "menu" entry); no Go constant is required. The named
// constants below exist only for the built-in layouts that carry special
// interaction behaviour (split's two-pane submit, fullscreen's tabs); any other
// layout id is treated as a simple chat-style layout.
type LayoutType string

type InfoView int

type FullscreenTab int

const MinSplitLayoutWidth = 90

const (
	minOutputPercentage     = 25
	maxOutputPercentage     = 75
	defaultOutputPercentage = 50
	outputPercentageStep    = 5
)

const (
	LayoutChat       LayoutType = "chat"
	LayoutSplit      LayoutType = "split"
	LayoutFullscreen LayoutType = "fullscreen"
	// LayoutZen is the built-in minimalistic single-column layout in neutral
	// greys. It needs no special Go behaviour — it's named only so the Go
	// fallback builder and default cycle can reference it.
	LayoutZen LayoutType = "zen"
)

const LayoutDefault = LayoutChat

const (
	InfoViewAI InfoView = iota
	InfoViewGitDiff
)

const (
	FullscreenTabEditor FullscreenTab = iota
	FullscreenTabOutput
	FullscreenTabDiff
	fullscreenTabCount
)

func defaultLayoutCycle() []LayoutType {
	return []LayoutType{LayoutChat, LayoutSplit, LayoutZen, LayoutFullscreen}
}

func (s *State) SetAvailableLayouts(layouts []LayoutType) {
	seen := map[LayoutType]bool{}
	available := make([]LayoutType, 0, len(layouts))
	for _, layoutType := range layouts {
		if seen[layoutType] {
			continue
		}
		seen[layoutType] = true
		available = append(available, layoutType)
	}
	if len(available) == 0 {
		available = defaultLayoutCycle()
	}
	s.availableLayouts = available
	for _, layoutType := range available {
		if s.Layout == layoutType {
			return
		}
	}
	s.Layout = available[0]
	s.invalidateOutputMax()
}

func (s *State) EffectiveLayout() LayoutType {
	return s.Layout
}

func (s *State) CycleInfoView() {
	s.outputScroll = 0
	s.InfoView = (s.InfoView + 1) % 2
	s.invalidateOutputMax()
	s.ChatLog().ClearSelection()
	s.DiffLog().ClearSelection()
	if s.InfoView == InfoViewGitDiff {
		s.RefreshGitDiff()
	}
}

func (s *State) CycleFullscreenTab(delta int) {
	before := s.FullscreenTab
	next := int(s.FullscreenTab) + delta
	for next < 0 {
		next += int(fullscreenTabCount)
	}
	s.FullscreenTab = FullscreenTab(next % int(fullscreenTabCount))
	if s.FullscreenTab != before {
		s.outputScroll = 0
		s.invalidateOutputMax()
		s.ChatLog().ClearSelection()
		s.DiffLog().ClearSelection()
	}
	if s.FullscreenTab == FullscreenTabDiff {
		s.RefreshGitDiff()
	}
}

func (s *State) FullscreenShowsEditor() bool {
	return s.EffectiveLayout() != LayoutFullscreen || s.FullscreenTab == FullscreenTabEditor
}

func (s *State) FullscreenOutputView() InfoView {
	if s.FullscreenTab == FullscreenTabDiff {
		return InfoViewGitDiff
	}
	return InfoViewAI
}

func (s *State) SwitchLayout() {
	if len(s.availableLayouts) == 0 {
		s.availableLayouts = defaultLayoutCycle()
	}
	next := 0
	for i, layoutType := range s.availableLayouts {
		if s.Layout == layoutType {
			next = (i + 1) % len(s.availableLayouts)
			break
		}
	}
	s.Layout = s.availableLayouts[next]
	if s.Layout == LayoutFullscreen {
		s.FullscreenTab = FullscreenTabEditor
	}
	s.invalidateOutputMax()
	s.ChatLog().ClearSelection()
	s.DiffLog().ClearSelection()
}

func (s *State) LayoutLabel() string {
	return layoutLabel(s.Layout)
}

func (s *State) SplitOutputPercent() int {
	min, max, _ := s.outputAdjustment()
	if s.OutputPercent == 0 {
		return defaultOutputPercentage
	}
	return clampInt(s.OutputPercent, min, max)
}

func (s *State) SplitLeftPercent() int {
	return 100 - s.SplitOutputPercent()
}

func (s *State) SplitLeftWidth() int {
	return int(float64(s.width) * float64(s.SplitLeftPercent()) / 100)
}

func (s *State) AdjustOutputPercentage(delta int) bool {
	min, max, _ := s.outputAdjustment()
	before := s.SplitOutputPercent()
	s.OutputPercent = clampInt(before+delta, min, max)
	if s.OutputPercent != before {
		s.invalidateOutputMax()
	}
	return s.OutputPercent != before
}

func (s *State) SetLayoutItems() {
	if len(s.availableLayouts) == 0 {
		s.availableLayouts = defaultLayoutCycle()
	}
	s.PaletteView = PaletteViewLayouts
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	for i, layoutType := range s.availableLayouts {
		if layoutType == s.Layout {
			s.PaletteIndex = i
			break
		}
	}
	s.PopupOpen = true
}

func (s *State) SelectedLayout() (LayoutType, bool) {
	layouts := s.visibleLayoutItems()
	if s.PaletteView != PaletteViewLayouts || s.PaletteIndex < 0 || s.PaletteIndex >= len(layouts) {
		return LayoutDefault, false
	}
	return layouts[s.PaletteIndex], true
}

func (s *State) SetLayout(layoutType LayoutType) {
	if s.Layout == layoutType {
		return
	}
	s.Layout = layoutType
	if s.Layout == LayoutFullscreen {
		s.FullscreenTab = FullscreenTabEditor
	}
	s.invalidateOutputMax()
	s.ChatLog().ClearSelection()
	s.DiffLog().ClearSelection()
}

func (s *State) visibleLayoutItems() []LayoutType {
	if len(s.availableLayouts) == 0 {
		s.availableLayouts = defaultLayoutCycle()
	}
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.availableLayouts
	}
	items := make([]LayoutType, 0, len(s.availableLayouts))
	for _, layoutType := range s.availableLayouts {
		if paletteMatches(query, layoutTitle(layoutType), layoutLabel(layoutType)) {
			items = append(items, layoutType)
		}
	}
	return items
}

// layoutLabel is the lower-case display label — just the layout id itself.
func layoutLabel(layoutType LayoutType) string {
	if layoutType == "" {
		return "unknown"
	}
	return string(layoutType)
}

// LayoutTypeFromString parses a layout name into a LayoutType. Any non-empty
// name is a valid id (validity against the config is enforced where layouts are
// listed); an empty name falls back to the default.
func LayoutTypeFromString(name string) LayoutType {
	if name == "" {
		return LayoutDefault
	}
	return LayoutType(name)
}

// layoutTitle is the human-facing title: the id with its first letter
// upper-cased (chat → Chat, zen → Zen).
func layoutTitle(layoutType LayoutType) string {
	s := string(layoutType)
	if s == "" {
		return "Unknown"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// LayoutScrollsOutput reports whether vertical scroll keys (PageUp/Down and the
// mouse wheel) should move the chat output rather than the editor. True for
// simple single-column layouts — chat, zen, and any custom layout that is
// neither split nor fullscreen.
func (s *State) LayoutScrollsOutput() bool {
	switch s.EffectiveLayout() {
	case LayoutSplit, LayoutFullscreen:
		return false
	default:
		return true
	}
}
