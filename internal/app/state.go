package app

import "plums/internal/components"

type LayoutType int

const (
	LayoutDefault LayoutType = iota
	LayoutFullscreen
	LayoutSplit
)

type State struct {
	width  int
	height int

	history []string
	TextBox *components.TextBox

	aioutput string

	Layout LayoutType
}

func NewState(width int, height int) *State {
	tb := components.NewTextBox()
	tb.SetMultiline(false)
	return &State{
		width:   width,
		height:  height,
		TextBox: tb,
	}
}

func (s *State) SubmitInput() {
	input := s.TextBox.GetContent()
	if input != "" {
		s.history = append(s.history, input)
		s.TextBox.SetContent("")
		s.TextBox.SetCursor(0)
		s.TextBox.ClearSelection()
	}
}

func (s *State) AppendAiOutput(b string) {
	s.aioutput += b
}

func (s *State) History() []string {
	return s.history
}

func (s *State) SwitchLayout() {
	switch s.Layout {
	case LayoutDefault:
		s.Layout = LayoutSplit
		s.TextBox.SetMultiline(true)
	case LayoutSplit:
		s.Layout = LayoutFullscreen
		s.TextBox.SetMultiline(true)
	case LayoutFullscreen:
		s.Layout = LayoutDefault
		s.TextBox.SetMultiline(false)
	default:
		s.Layout = LayoutDefault
		s.TextBox.SetMultiline(false)
	}
}
