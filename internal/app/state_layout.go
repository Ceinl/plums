package app

import ()

type LayoutType int

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
	LayoutChat LayoutType = iota
	LayoutSplit
	LayoutFullscreen
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
	return []LayoutType{LayoutChat, LayoutSplit, LayoutFullscreen}
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

func layoutLabel(layoutType LayoutType) string {
	switch layoutType {
	case LayoutChat:
		return "chat"
	case LayoutSplit:
		return "split"
	case LayoutFullscreen:
		return "fullscreen"
	default:
		return "unknown"
	}
}

// LayoutTypeFromString parses a layout name into a LayoutType.
func LayoutTypeFromString(name string) LayoutType {
	switch name {
	case "chat":
		return LayoutChat
	case "split":
		return LayoutSplit
	case "fullscreen":
		return LayoutFullscreen
	default:
		return LayoutDefault
	}
}

func layoutTitle(layoutType LayoutType) string {
	switch layoutType {
	case LayoutChat:
		return "Chat"
	case LayoutSplit:
		return "Split"
	case LayoutFullscreen:
		return "Fullscreen"
	default:
		return "Unknown"
	}
}
