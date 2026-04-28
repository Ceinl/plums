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

func (s *State) PopInput(b rune) {
	runes := []rune(s.input)
	s.input = string(runes[:len(runes)-1])
}

func (s *State) AppendAiOutput(b string) {
	s.aioutput += b
}
