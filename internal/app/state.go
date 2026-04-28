package app

type State struct {
	width  int
	height int

	history []string
	input   string

	aioutput string
}

func NewState(width int, height int) *State {
	return &State{
		width:  width,
		height: height,
	}
}

func (s *State) AppendInput(b rune) {
	s.input += string(b)
}

func (s *State) PopInput() {
	runes := []rune(s.input)
	if len(runes) > 0 {
		s.input = string(runes[:len(runes)-1])
	}
}

func (s *State) SubmitInput() {
	if s.input != "" {
		s.history = append(s.history, s.input)
		s.input = ""
	}
}

func (s *State) AppendAiOutput(b string) {
	s.aioutput += b
}

func (s *State) History() []string {
	return s.history
}
