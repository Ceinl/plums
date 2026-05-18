package app

import (
	"strings"

	"plums/internal/components"
)

type PaletteAction int

const (
	PaletteActionNone PaletteAction = iota
	PaletteActionChangeModel
	PaletteActionNewSession
	PaletteActionSwitchMode
	PaletteActionSessionsList
)

type LayoutType int

const MinSplitLayoutWidth = 100

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

	aioutput     string
	isStreaming  bool
	outputScroll int

	spinnerFrame int

	Layout LayoutType

	SessionID      string
	ServerStarting bool
	ServerReady    bool
	PopupOpen      bool
	PaletteIndex   int
	PendingAction  PaletteAction
	Mode           string
}

func NewState(width int, height int) *State {
	return &State{
		width:  width,
		height: height,
		Editor: components.NewTextEditor(),
		Layout: LayoutSplit,
		Mode:   "build",
	}
}

func (s *State) SubmitInput() string {
	input := s.Editor.GetContent()
	if s.runEditorCommand(input) {
		return ""
	}
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

func (s *State) EffectiveLayout() LayoutType {
	if s.Layout == LayoutSplit && s.width < MinSplitLayoutWidth {
		return LayoutDefault
	}
	return s.Layout
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

func (s *State) OutputScroll() int {
	return s.outputScroll
}

func (s *State) ScrollOutput(delta int) {
	s.outputScroll += delta
	if s.outputScroll < 0 {
		s.outputScroll = 0
	}
}

func (s *State) ScrollOutputPage(direction int) {
	page := s.height - 4
	if page < 1 {
		page = 1
	}
	s.ScrollOutput(direction * page)
}

func (s *State) ScrollOutputBottom() {
	s.outputScroll = 0
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

func (s *State) TogglePopup() {
	if s.PopupOpen {
		s.ClosePalette()
		return
	}
	s.OpenPalette()
}

func (s *State) OpenPalette() {
	s.PopupOpen = true
	s.PendingAction = PaletteActionNone
	if s.PaletteIndex >= len(s.PaletteItems()) {
		s.PaletteIndex = 0
	}
}

func (s *State) ClosePalette() {
	s.PopupOpen = false
	s.PendingAction = PaletteActionNone
}

func (s *State) PaletteItems() []components.PopupItem {
	modeLabel := "Switch to plan mode"
	if s.Mode == "plan" {
		modeLabel = "Switch to build mode"
	}
	return []components.PopupItem{
		{Title: "Change model", Detail: "Requires model API wiring", Disabled: true},
		{Title: "Start new session", Detail: "Create a fresh opencode session"},
		{Title: modeLabel, Detail: "Current mode: " + s.Mode},
		{Title: "Sessions list", Detail: "Later: attach to older sessions", Disabled: true},
	}
}

func (s *State) MovePalette(delta int) {
	items := s.PaletteItems()
	if len(items) == 0 {
		return
	}
	for range items {
		s.PaletteIndex = (s.PaletteIndex + delta + len(items)) % len(items)
		if !items[s.PaletteIndex].Disabled {
			return
		}
	}
}

func (s *State) SelectPaletteItem() {
	items := s.PaletteItems()
	if s.PaletteIndex < 0 || s.PaletteIndex >= len(items) || items[s.PaletteIndex].Disabled {
		return
	}
	s.PendingAction = []PaletteAction{
		PaletteActionChangeModel,
		PaletteActionNewSession,
		PaletteActionSwitchMode,
		PaletteActionSessionsList,
	}[s.PaletteIndex]
	s.PopupOpen = false
}

func (s *State) ConsumePendingAction() PaletteAction {
	action := s.PendingAction
	s.PendingAction = PaletteActionNone
	return action
}

func (s *State) SetSessionID(id string) {
	s.SessionID = id
}

func (s *State) ClearConversation() {
	s.messages = nil
	s.aioutput = ""
	s.outputScroll = 0
}

func (s *State) ToggleMode() {
	if s.Mode == "plan" {
		s.Mode = "build"
	} else {
		s.Mode = "plan"
	}
}

func (s *State) runEditorCommand(input string) bool {
	line := strings.TrimSpace(input)
	if !strings.HasPrefix(line, ">") {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, ">")))
	switch command {
	case "clear":
		s.Editor.SetContent("")
		return true
	default:
		return false
	}
}
