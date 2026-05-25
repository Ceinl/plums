package app

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"plums/internal/components"
)

type PaletteAction int

const (
	PaletteActionNone PaletteAction = iota
	PaletteActionChangeModel
	PaletteActionNewSession
	PaletteActionSwitchMode
	PaletteActionSessionsList
	PaletteActionSelectSession
)

type PaletteView int

const (
	PaletteViewCommands PaletteView = iota
	PaletteViewSessions
)

type LayoutType int

type InfoView int

const MinSplitLayoutWidth = 90

const (
	LayoutDefault LayoutType = iota
	LayoutFullscreen
	LayoutSplit
)

const (
	InfoViewAI InfoView = iota
	InfoViewGitDiff
)

type Message struct {
	Role    string
	Content string
}

type SessionListItem struct {
	ID      string
	Title   string
	Current bool
}

type SlashCommand struct {
	Name   string
	Detail string
}

var slashCommands = []SlashCommand{
	{Name: "/new", Detail: "Create a fresh opencode session"},
	{Name: "/command", Detail: "Open the command palette"},
	{Name: "/sessions", Detail: "Open existing opencode sessions"},
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
	SessionTitle   string
	ServerStarting bool
	ServerReady    bool
	PopupOpen      bool
	PaletteIndex   int
	PaletteView    PaletteView
	PendingAction  PaletteAction
	SessionItems   []SessionListItem
	Mode           string
	ModelProvider  string
	ModelID        string
	InfoView       InfoView
	GitDiff        string
	submittedInput string
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
	s.submittedInput = ""
	input := s.Editor.GetContent()
	if s.runEditorCommand(input) {
		return ""
	}
	if input != "" {
		s.messages = append(s.messages, Message{Role: "user", Content: input})
		s.Editor.SetContent("")
		s.submittedInput = input
	}
	return input
}

func (s *State) ConsumeSubmittedInput() string {
	input := s.submittedInput
	s.submittedInput = ""
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

func (s *State) ScrollOutput(delta int) bool {
	before := s.outputScroll
	s.outputScroll += delta
	if s.outputScroll < 0 {
		s.outputScroll = 0
	}
	return s.outputScroll != before
}

func (s *State) ScrollOutputVisible(delta int) bool {
	before := s.outputScroll
	s.ScrollOutput(delta)
	s.ClampOutputScroll(maxOutputScroll(s))
	return s.outputScroll != before
}

func (s *State) ScrollAt(x, y, delta int) bool {
	if s.isEditorPoint(x, y) {
		return s.Editor.Scroll(delta)
	}
	return s.ScrollOutputVisible(delta)
}

func (s *State) isEditorPoint(x, y int) bool {
	if x < 0 || y < 0 || x >= s.width || y >= s.height {
		return false
	}

	switch s.EffectiveLayout() {
	case LayoutFullscreen:
		return true
	case LayoutSplit:
		if s.width >= MinSplitLayoutWidth {
			leftW := int(float64(s.width) * 0.5)
			return x < leftW && !s.PopupOpen
		}
		outputH := int(float64(s.height) * 0.5)
		return y > outputH
	case LayoutDefault:
		return y >= s.height-5
	default:
		return false
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

func (s *State) CycleInfoView() {
	s.outputScroll = 0
	s.InfoView = (s.InfoView + 1) % 2
	if s.InfoView == InfoViewGitDiff {
		s.RefreshGitDiff()
	}
}

func (s *State) RefreshGitDiff() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "diff", "--", ".").CombinedOutput()
	if err != nil {
		s.GitDiff = strings.TrimSpace(string(out))
		if s.GitDiff != "" {
			s.GitDiff += "\n"
		}
		if ctx.Err() == context.DeadlineExceeded {
			s.GitDiff += "git diff timed out"
			return
		}
		s.GitDiff += err.Error()
		return
	}
	s.GitDiff = string(out)
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
	s.PaletteView = PaletteViewCommands
	s.PendingAction = PaletteActionNone
	items := s.PaletteItems()
	if s.PaletteIndex >= len(items) {
		s.PaletteIndex = 0
	}
	if len(items) > 0 && items[s.PaletteIndex].Disabled {
		s.MovePalette(1)
	}
}

func (s *State) ClosePalette() {
	s.PopupOpen = false
	s.PaletteView = PaletteViewCommands
	s.PendingAction = PaletteActionNone
}

func (s *State) PaletteTitle() string {
	if s.PaletteView == PaletteViewSessions {
		return "Sessions"
	}
	return "Command Palette"
}

func (s *State) PaletteItems() []components.PopupItem {
	if s.PaletteView == PaletteViewSessions {
		if len(s.SessionItems) == 0 {
			return []components.PopupItem{{Title: "No sessions", Detail: "No opencode sessions found", Disabled: true}}
		}
		items := make([]components.PopupItem, len(s.SessionItems))
		for i, session := range s.SessionItems {
			title := session.Title
			if title == "" {
				title = session.ID
			}
			detail := session.ID
			if session.Current {
				detail = "current - " + session.ID
			}
			items[i] = components.PopupItem{Title: title, Detail: detail}
		}
		return items
	}
	modeLabel := "Switch to plan mode"
	if s.Mode == "plan" {
		modeLabel = "Switch to build mode"
	}
	return []components.PopupItem{
		{Title: "Change model", Detail: "Requires model API wiring", Disabled: true},
		{Title: "Start new session", Detail: "Create a fresh opencode session"},
		{Title: modeLabel, Detail: "Current mode: " + s.Mode},
		{Title: "Sessions list", Detail: "Open existing opencode sessions"},
	}
}

func (s *State) SlashCommands() []SlashCommand {
	input := s.Editor.GetContent()
	if !strings.HasPrefix(input, "/") || strings.Contains(input, "\n") {
		return nil
	}

	items := make([]SlashCommand, 0, len(slashCommands))
	for _, command := range slashCommands {
		if strings.HasPrefix(command.Name, input) {
			items = append(items, command)
		}
	}
	return items
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
	if s.PaletteView == PaletteViewSessions {
		s.PendingAction = PaletteActionSelectSession
		s.PopupOpen = false
		return
	}
	s.PendingAction = []PaletteAction{
		PaletteActionChangeModel,
		PaletteActionNewSession,
		PaletteActionSwitchMode,
		PaletteActionSessionsList,
	}[s.PaletteIndex]
	if s.PendingAction != PaletteActionSessionsList {
		s.PopupOpen = false
	}
}

func (s *State) ConsumePendingAction() PaletteAction {
	action := s.PendingAction
	s.PendingAction = PaletteActionNone
	return action
}

func (s *State) SetSessionID(id string) {
	s.SessionID = id
}

func (s *State) SetSessionTitle(title string) {
	s.SessionTitle = title
}

func (s *State) SetSessionItems(items []SessionListItem) {
	s.SessionItems = items
	s.PaletteView = PaletteViewSessions
	s.PaletteIndex = 0
	if len(items) > 0 {
		for i, item := range items {
			if item.ID == s.SessionID {
				s.PaletteIndex = i
				break
			}
		}
	}
	s.PopupOpen = true
}

func (s *State) SelectedSessionID() string {
	if s.PaletteView != PaletteViewSessions || s.PaletteIndex < 0 || s.PaletteIndex >= len(s.SessionItems) {
		return ""
	}
	return s.SessionItems[s.PaletteIndex].ID
}

func (s *State) SetModel(providerID, modelID string) {
	s.ModelProvider = providerID
	s.ModelID = modelID
}

func (s *State) ClearConversation() {
	s.messages = nil
	s.aioutput = ""
	s.outputScroll = 0
}

func (s *State) SetConversation(messages []Message) {
	s.messages = messages
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
	if !strings.HasPrefix(line, ">") && !strings.HasPrefix(line, "/") {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(line[1:]))
	switch command {
	case "clear":
		s.Editor.SetContent("")
		return true
	case "command":
		s.Editor.SetContent("")
		s.OpenPalette()
		return true
	case "new":
		s.Editor.SetContent("")
		s.PendingAction = PaletteActionNewSession
		return true
	case "sessions":
		s.Editor.SetContent("")
		s.PendingAction = PaletteActionSessionsList
		return true
	default:
		return false
	}
}
