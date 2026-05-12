package app

import "plums/internal/components"

type LayoutType int

const (
	LayoutDefault LayoutType = iota
	LayoutFullscreen
	LayoutSplit
)

type Message struct {
	Role    string
	Content string
}

type State struct {
	width  int
	height int

	messages []Message
	TextBox  *components.TextBox

	aioutput    string
	isStreaming bool

	Layout LayoutType

	SessionID string
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

func (s *State) SubmitInput() string {
	input := s.TextBox.GetContent()
	if input != "" {
		s.messages = append(s.messages, Message{Role: "user", Content: input})
		s.TextBox.SetContent("")
		s.TextBox.SetCursor(0)
		s.TextBox.ClearSelection()
	}
	return input
}

func (s *State) AppendAiOutput(b string) {
	s.aioutput += b
}

func (s *State) ClearAiOutput() {
	s.aioutput = ""
}

func (s *State) SetStreaming(v bool) {
	s.isStreaming = v
}

func (s *State) FinalizeAiOutput() {
	if s.aioutput != "" {
		s.messages = append(s.messages, Message{Role: "ai", Content: s.aioutput})
		s.aioutput = ""
		s.isStreaming = false
	}
}

func (s *State) Messages() []Message {
	return s.messages
}

func (s *State) Resize(w, h int) {
	s.width = w
	s.height = h
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
