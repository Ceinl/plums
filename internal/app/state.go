package app

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

type Message struct {
	Role    string
	Content string
}

type SessionListItem struct {
	ID        string
	Title     string
	Directory string
	Updated   int64
	Current   bool
}

type ModelListItem struct {
	ProviderID   string
	ProviderName string
	ModelID      string
	ModelName    string
	Current      bool
}

type BackendListItem struct {
	ID      string
	Name    string
	Current bool
}

type State struct {
	width  int
	height int

	messages []Message
	Editor   *components.Editor

	aioutput     string
	isStreaming  bool
	outputScroll int
	outputMax    int
	outputMaxSet bool

	spinnerFrame int

	Layout           LayoutType
	availableLayouts []LayoutType

	SessionID          string
	SessionTitle       string
	ServerStarting     bool
	ServerReady        bool
	PopupOpen          bool
	PaletteIndex       int
	DropdownIndex      int
	DropdownHidden     bool
	PaletteView        PaletteView
	PaletteQuery       string
	PendingAction      PaletteAction
	ModelItems         []ModelListItem
	SessionItems       []SessionListItem
	SkillItems         []SkillListItem
	QuestionTitle      string
	QuestionItems      []QuestionOptionItem
	BackendItems       []BackendListItem
	ListTitle          string
	ListItems          []capabilities.ListItem
	listOnPick         func(capabilities.ListItem)
	BackendProvider    string
	Mode               string
	Theme              capabilities.Theme
	ThinkingMode       components.ThinkingVisibility
	ToolCallMode       components.ToolCallVisibility
	ModelProvider      string
	ModelID            string
	InfoView           InfoView
	GitDiff            string
	OutputPercent      int
	submittedInput     string
	submittedMessage   string
	pendingCommand     string
	commands           []capabilities.Command
	componentFactories map[string]ComponentFactory
	publicComponents   []*publicComponentAdapter
	publicInstances    map[string]capabilities.Component
	mouseCapture       *publicComponentAdapter
	projectFiles       []string
	completion         *completionRegistry

	// runSessionIDs tracks sessions created during this run, so pre-existing
	// backend history can be hidden when RunConfig.ClearHistory is set.
	runSessionIDs map[string]bool

	chatLog            *components.ChatLog
	diffLog            *components.DiffLog
	sessions           *components.Sessions
	sessionsHorizontal *components.Sessions

	outputMouseSelecting bool
}

func NewState(width int, height int) *State {
	width, height = clampSize(width, height)
	return &State{
		width:            width,
		height:           height,
		Editor:           components.NewTextEditor(),
		Layout:           LayoutSplit,
		availableLayouts: defaultLayoutCycle(),
		Mode:             "build",
		OutputPercent:    defaultOutputPercentage,
		ThinkingMode:     components.ThinkingVisibilityHidden,
		ToolCallMode:     components.ToolCallVisibilityFull,
		runSessionIDs:    map[string]bool{},
	}
}

// MarkRunSession records a session as created during this run.
func (s *State) MarkRunSession(id string) {
	if id == "" {
		return
	}
	s.runSessionIDs[id] = true
}

// IsRunSession reports whether the session was created during this run.
func (s *State) IsRunSession(id string) bool {
	return s.runSessionIDs[id]
}

func (s *State) SubmitInput() string {
	s.submittedInput = ""
	s.submittedMessage = ""
	input := s.Editor.GetContent()
	if s.runEditorCommand(input) {
		return ""
	}
	return s.SubmitPrompt(input)
}

func (s *State) SubmitPrompt(input string) string {
	s.submittedInput = ""
	s.submittedMessage = ""
	if input != "" {
		s.messages = append(s.messages, Message{Role: "user", Content: input})
		s.Editor.SetContent("")
		s.submittedInput = ExpandSkillMarkers(input, s.SkillItems)
		s.submittedMessage = input
		s.invalidateOutputMax()
	}
	return input
}

func (s *State) ConsumeSubmittedInput() string {
	input := s.submittedInput
	s.submittedInput = ""
	return input
}

func (s *State) ConsumeSubmittedMessage() string {
	message := s.submittedMessage
	s.submittedMessage = ""
	return message
}

func (s *State) AppendAiOutput(b string) {
	s.aioutput += b
	s.invalidateOutputMax()
}

func (s *State) ClearAiOutput() {
	s.aioutput = ""
	s.invalidateOutputMax()
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

func (s *State) FinalizeAiOutput() string {
	s.isStreaming = false
	if s.aioutput != "" {
		output := s.aioutput
		s.messages = append(s.messages, Message{Role: "ai", Content: s.aioutput})
		s.aioutput = ""
		s.invalidateOutputMax()
		return output
	}
	return ""
}

func (s *State) Messages() []Message {
	return s.messages
}

func (s *State) Resize(w, h int) {
	w, h = clampSize(w, h)
	s.width = w
	s.height = h
	s.invalidateOutputMax()
}

func clampSize(w, h int) (int, int) {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
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
			s.invalidateOutputMax()
			return
		}
		s.GitDiff += err.Error()
		s.invalidateOutputMax()
		return
	}
	s.GitDiff = string(out)
	s.invalidateOutputMax()
}

// AddMessage appends a message with the given role directly to the log.
// Use role "system" for status / error notices.
func (s *State) AddMessage(role, content string) {
	s.messages = append(s.messages, Message{Role: role, Content: content})
	s.invalidateOutputMax()
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func (s *State) SetSessionID(id string) {
	s.SessionID = id
}

func (s *State) SetSessionTitle(title string) {
	s.SessionTitle = title
}

func (s *State) SetBackendProvider(provider string) {
	s.BackendProvider = provider
}

func (s *State) SetModel(providerID, modelID string) {
	s.ModelProvider = providerID
	s.ModelID = modelID
}

func (s *State) SetTheme(value capabilities.Theme) {
	value.Name = strings.ToLower(strings.TrimSpace(value.Name))
	s.Theme = value
}

func (s *State) ThemeName() string {
	return strings.ToLower(strings.TrimSpace(s.Theme.Name))
}

func (s *State) EffectiveTheme() capabilities.Theme {
	if name := s.ThemeName(); name != "" {
		return capabilities.Theme{Name: name}
	}
	return capabilities.Theme{Name: "default"}
}

func (s *State) BeginPublicComponentFrame() {
	s.publicComponents = nil
}

func (s *State) addPublicComponent(component *publicComponentAdapter) {
	if component == nil {
		return
	}
	s.publicComponents = append(s.publicComponents, component)
}

// publicComponentInstance returns a per-build instance for a component template,
// cached by name so the instance's private render/input state survives layout
// rebuilds (the factory is re-invoked on every rebuild).
func (s *State) publicComponentInstance(name string, build func() capabilities.Component) capabilities.Component {
	if s.publicInstances == nil {
		s.publicInstances = make(map[string]capabilities.Component)
	}
	if existing, ok := s.publicInstances[name]; ok {
		return existing
	}
	instance := build()
	s.publicInstances[name] = instance
	return instance
}

func (s *State) PublicComponentSelection() string {
	for i := len(s.publicComponents) - 1; i >= 0; i-- {
		if selection := s.publicComponents[i].Selection(); selection != "" {
			return selection
		}
	}
	return ""
}

func (s *State) ResetBackendSession() {
	s.SessionID = ""
	s.SessionTitle = ""
	s.ServerReady = false
	s.ModelProvider = ""
	s.ModelID = ""
	s.aioutput = ""
	s.isStreaming = false
	s.outputScroll = 0
	s.invalidateOutputMax()
}

func (s *State) ClearConversation() {
	s.messages = nil
	s.aioutput = ""
	s.outputScroll = 0
	s.invalidateOutputMax()
	s.ChatLog().ClearSelection()
}

func (s *State) SetConversation(messages []Message) {
	s.messages = messages
	s.aioutput = ""
	s.outputScroll = 0
	s.invalidateOutputMax()
	s.ChatLog().ClearSelection()
}

func (s *State) ChatLog() *components.ChatLog {
	if s.chatLog == nil {
		s.chatLog = components.NewChatLog()
		s.chatLog.SetMaxScrollObserver(s.SetOutputMaxScroll)
	}
	return s.chatLog
}

func (s *State) DiffLog() *components.DiffLog {
	if s.diffLog == nil {
		s.diffLog = components.NewDiffLog()
		s.diffLog.SetMaxScrollObserver(s.SetOutputMaxScroll)
	}
	return s.diffLog
}

func (s *State) Sessions() *components.Sessions {
	if s.sessions == nil {
		s.sessions = components.NewSessions(components.SessionsVertical)
	}
	return s.sessions
}

func (s *State) SessionsHorizontal() *components.Sessions {
	if s.sessionsHorizontal == nil {
		s.sessionsHorizontal = components.NewSessions(components.SessionsHorizontal)
	}
	return s.sessionsHorizontal
}

func (s *State) ThinkingVisibilityLabel() string {
	switch s.ThinkingMode {
	case components.ThinkingVisibilityHidden:
		return "hidden"
	case components.ThinkingVisibilityTitle:
		return "title"
	default:
		return "full"
	}
}

func (s *State) CycleThinkingVisibility() {
	switch s.ThinkingMode {
	case components.ThinkingVisibilityFull:
		s.ThinkingMode = components.ThinkingVisibilityTitle
	case components.ThinkingVisibilityTitle:
		s.ThinkingMode = components.ThinkingVisibilityHidden
	default:
		s.ThinkingMode = components.ThinkingVisibilityFull
	}
	s.invalidateOutputMax()
	s.ChatLog().SetThinkingVisibility(s.ThinkingMode)
}

func (s *State) ToolCallVisibilityLabel() string {
	switch s.ToolCallMode {
	case components.ToolCallVisibilityHidden:
		return "hidden"
	case components.ToolCallVisibilityCollapse:
		return "collapse"
	default:
		return "full"
	}
}

func (s *State) CycleToolCallVisibility() {
	switch s.ToolCallMode {
	case components.ToolCallVisibilityFull:
		s.ToolCallMode = components.ToolCallVisibilityCollapse
	case components.ToolCallVisibilityCollapse:
		s.ToolCallMode = components.ToolCallVisibilityHidden
	default:
		s.ToolCallMode = components.ToolCallVisibilityFull
	}
	s.invalidateOutputMax()
	s.ChatLog().SetToolCallVisibility(s.ToolCallMode)
}

func (s *State) ToggleMode() {
	if s.Mode == "plan" {
		s.Mode = "build"
	} else {
		s.Mode = "plan"
	}
}
