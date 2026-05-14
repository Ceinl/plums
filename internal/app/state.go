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
	Editor   *components.Editor

	aioutput    string
	isStreaming bool

	Layout LayoutType

	SessionID string
}

func NewState(width int, height int) *State {
	return &State{
		width:  width,
		height: height,
		Editor: components.NewTextEditor(),
	}
}

func (s *State) SubmitInput() string {
	input := s.Editor.GetContent()
	if input != "" {
		s.messages = append(s.messages, Message{Role: "user", Content: input})
		s.Editor.SetContent("")
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
	case LayoutSplit:
		s.Layout = LayoutFullscreen
	case LayoutFullscreen:
		s.Layout = LayoutDefault
	default:
		s.Layout = LayoutDefault
	}
}
