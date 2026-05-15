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

	spinnerFrame int

	Layout LayoutType

	SessionID      string
	ServerStarting bool
	ServerReady    bool
}

func NewState(width int, height int) *State {
	return &State{
		width:  width,
		height: height,
		Editor: components.NewTextEditor(),
		Layout: LayoutSplit,
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

func (s *State) SetServerStarting(v bool) {
	s.ServerStarting = v
}

func (s *State) SetServerReady(v bool) {
	s.ServerReady = v
}

func (s *State) FinalizeAiOutput() {
	s.isStreaming = false
	if s.aioutput != "" {
		s.messages = append(s.messages, Message{Role: "ai", Content: s.aioutput})
		s.aioutput = ""
	}
}

func (s *State) Messages() []Message {
	return s.messages
}

func (s *State) Resize(w, h int) {
	s.width = w
	s.height = h
}

func (s *State) IsStreaming() bool {
	return s.isStreaming
}

func (s *State) SpinnerFrame() int {
	return s.spinnerFrame
}

func (s *State) TickSpinner() {
	s.spinnerFrame = (s.spinnerFrame + 1) % 10
}

// AddMessage appends a message with the given role directly to the log.
// Use role "system" for status / error notices.
func (s *State) AddMessage(role, content string) {
	s.messages = append(s.messages, Message{Role: role, Content: content})
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
