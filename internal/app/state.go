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
	PaletteActionOpenPalette
	PaletteActionChangeModel
	PaletteActionSelectModel
	PaletteActionNewSession
	PaletteActionSwitchMode
	PaletteActionSessionsList
	PaletteActionSelectSession
	PaletteActionSkillsList
	PaletteActionSelectSkill
)

type PaletteView int

const (
	PaletteViewCommands PaletteView = iota
	PaletteViewModels
	PaletteViewSessions
	PaletteViewSkills
)

type LayoutType int

type InfoView int

const MinSplitLayoutWidth = 90

const (
	minOutputPercentage     = 25
	maxOutputPercentage     = 75
	defaultOutputPercentage = 50
	outputPercentageStep    = 5
)

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

type ModelListItem struct {
	ProviderID   string
	ProviderName string
	ModelID      string
	ModelName    string
	Current      bool
}

type paletteCommandItem struct {
	Title    string
	Detail   string
	Action   PaletteAction
	Adjust   bool
	Min      int
	Max      int
	Step     int
	Disabled bool
}

type SlashCommand struct {
	Name   string
	Detail string
	Action PaletteAction
}

type SkillSuggestion struct {
	Name        string
	Description string
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

	Layout LayoutType

	SessionID      string
	SessionTitle   string
	ServerStarting bool
	ServerReady    bool
	PopupOpen      bool
	PaletteIndex   int
	PaletteView    PaletteView
	PaletteQuery   string
	PendingAction  PaletteAction
	ModelItems     []ModelListItem
	SessionItems   []SessionListItem
	SkillItems     []SkillListItem
	Mode           string
	ModelProvider  string
	ModelID        string
	InfoView       InfoView
	GitDiff        string
	OutputPercent  int
	submittedInput string
	commandConfig  *CommandConfig
}

func NewState(width int, height int) *State {
	width, height = clampSize(width, height)
	return &State{
		width:         width,
		height:        height,
		Editor:        components.NewTextEditor(),
		Layout:        LayoutSplit,
		Mode:          "build",
		OutputPercent: defaultOutputPercentage,
		commandConfig: DefaultCommandConfig(),
	}
}

func (s *State) SetCommandConfig(cfg *CommandConfig) {
	if cfg == nil {
		cfg = DefaultCommandConfig()
	}
	s.commandConfig = cfg
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
		s.submittedInput = ExpandSkillMarkers(input, s.SkillItems)
		s.invalidateOutputMax()
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

func (s *State) FinalizeAiOutput() {
	s.isStreaming = false
	if s.aioutput != "" {
		s.messages = append(s.messages, Message{Role: "ai", Content: s.aioutput})
		s.aioutput = ""
		s.invalidateOutputMax()
	}
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
	if s.outputMaxSet {
		s.ClampOutputScroll(s.outputMax)
	}
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
			leftW := s.SplitLeftWidth()
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

func (s *State) SetOutputMaxScroll(maxOffset int) {
	s.outputMax = maxOffset
	s.outputMaxSet = true
	s.ClampOutputScroll(maxOffset)
}

func (s *State) invalidateOutputMax() {
	s.outputMaxSet = false
}

func (s *State) CycleInfoView() {
	s.outputScroll = 0
	s.InfoView = (s.InfoView + 1) % 2
	s.invalidateOutputMax()
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
	s.invalidateOutputMax()
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
	s.PaletteQuery = ""
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
	s.PaletteQuery = ""
	s.PendingAction = PaletteActionNone
}

func (s *State) PaletteTitle() string {
	if s.PaletteView == PaletteViewModels {
		return "Models"
	}
	if s.PaletteView == PaletteViewSessions {
		return "Sessions"
	}
	if s.PaletteView == PaletteViewSkills {
		return "Skills"
	}
	return s.commandConfig.Palette.Title
}

func (s *State) PaletteItems() []components.PopupItem {
	if s.PaletteView == PaletteViewModels {
		models := s.visibleModelItems()
		if len(models) == 0 {
			return []components.PopupItem{{Title: "No models", Detail: "No opencode models found", Disabled: true}}
		}
		items := make([]components.PopupItem, len(models))
		for i, model := range models {
			title := model.ModelName
			if title == "" {
				title = model.ModelID
			}
			detail := model.ProviderID + "/" + model.ModelID
			if model.ProviderName != "" {
				detail = model.ProviderName + " - " + detail
			}
			if model.Current {
				detail = "current - " + detail
			}
			items[i] = components.PopupItem{Title: title, Detail: detail}
		}
		return items
	}
	if s.PaletteView == PaletteViewSessions {
		sessions := s.visibleSessionItems()
		if len(sessions) == 0 {
			return []components.PopupItem{{Title: s.commandConfig.Palette.EmptySessionsTitle, Detail: s.commandConfig.Palette.EmptySessionsDetail, Disabled: true}}
		}
		items := make([]components.PopupItem, len(sessions))
		for i, session := range sessions {
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
	if s.PaletteView == PaletteViewSkills {
		skills := s.visibleSkillItems()
		if len(skills) == 0 {
			return []components.PopupItem{{Title: "No skills", Detail: "No opencode skills found", Disabled: true}}
		}
		items := make([]components.PopupItem, len(skills))
		for i, skill := range skills {
			items[i] = components.PopupItem{Title: skill.Name, Detail: skill.Description}
		}
		return items
	}
	commands := s.visibleCommandItems()
	if len(commands) == 0 {
		return []components.PopupItem{{Title: "No commands", Detail: "No matching commands", Disabled: true}}
	}
	items := make([]components.PopupItem, len(commands))
	for i, command := range commands {
		items[i] = components.PopupItem{Title: command.Title, Detail: command.Detail, Disabled: command.Disabled}
	}
	return items
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

func (s *State) AdjustSelectedPaletteItem(delta int) bool {
	command, ok := s.selectedAdjustableCommand()
	if !ok {
		return false
	}
	step := command.Step
	if step == 0 {
		_, _, step = s.outputAdjustment()
	}
	return s.AdjustOutputPercentage(delta * step)
}

func (s *State) IsOutputPercentageSelected() bool {
	_, ok := s.selectedAdjustableCommand()
	return ok
}

func (s *State) selectedAdjustableCommand() (paletteCommandItem, bool) {
	if s.PaletteView != PaletteViewCommands {
		return paletteCommandItem{}, false
	}
	commands := s.visibleCommandItems()
	if s.PaletteIndex < 0 || s.PaletteIndex >= len(commands) || !commands[s.PaletteIndex].Adjust {
		return paletteCommandItem{}, false
	}
	return commands[s.PaletteIndex], true
}

func (s *State) outputAdjustment() (min, max, step int) {
	min, max, step = minOutputPercentage, maxOutputPercentage, outputPercentageStep
	if s.commandConfig == nil {
		return min, max, step
	}
	for _, spec := range s.commandConfig.Actions {
		if spec.Adjustment.Step == 0 {
			continue
		}
		if spec.Adjustment.Min != 0 {
			min = spec.Adjustment.Min
		}
		if spec.Adjustment.Max != 0 {
			max = spec.Adjustment.Max
		}
		step = spec.Adjustment.Step
		return min, max, step
	}
	return min, max, step
}

func (s *State) PaletteSearch() string {
	return s.PaletteQuery
}

func (s *State) InsertPaletteRune(ch rune) {
	if ch < 32 || ch == 127 {
		return
	}
	s.PaletteQuery += string(ch)
	s.PaletteIndex = 0
	s.ensurePaletteSelection()
}

func (s *State) DeletePaletteRune() bool {
	if s.PaletteQuery == "" {
		return false
	}
	runes := []rune(s.PaletteQuery)
	s.PaletteQuery = string(runes[:len(runes)-1])
	s.PaletteIndex = 0
	s.ensurePaletteSelection()
	return true
}

func (s *State) ClearPaletteSearch() bool {
	if s.PaletteQuery == "" {
		return false
	}
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	s.ensurePaletteSelection()
	return true
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

func (s *State) SlashCommands() []SlashCommand {
	input := s.Editor.GetContent()
	if !strings.HasPrefix(input, "/") || strings.Contains(input, "\n") {
		return nil
	}

	items := make([]SlashCommand, 0, len(s.commandConfig.SlashCommands))
	for _, command := range s.commandConfig.SlashCommands {
		if strings.HasPrefix(command.Name, input) {
			items = append(items, SlashCommand{Name: command.Name, Detail: command.Detail, Action: s.commandConfig.actionFor(command.Action)})
		}
	}
	return items
}

func (s *State) SkillSuggestions() []SkillSuggestion {
	input := s.Editor.GetContent()
	if strings.Contains(input, "\n") || !strings.HasPrefix(input, "/skill ") {
		return nil
	}
	query := normalizedQuery(strings.TrimPrefix(input, "/skill "))
	items := make([]SkillSuggestion, 0, len(s.SkillItems))
	for _, skill := range s.SkillItems {
		if query == "" || paletteMatches(query, skill.Name, skill.Description) {
			items = append(items, SkillSuggestion{Name: skill.Name, Description: skill.Description})
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

func (s *State) ensurePaletteSelection() {
	items := s.PaletteItems()
	if len(items) == 0 {
		s.PaletteIndex = 0
		return
	}
	if s.PaletteIndex >= len(items) {
		s.PaletteIndex = len(items) - 1
	}
	if s.PaletteIndex < 0 {
		s.PaletteIndex = 0
	}
	if items[s.PaletteIndex].Disabled {
		s.MovePalette(1)
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
	if s.PaletteView == PaletteViewModels {
		s.PendingAction = PaletteActionSelectModel
		s.PopupOpen = false
		return
	}
	if s.PaletteView == PaletteViewSkills {
		s.PendingAction = PaletteActionSelectSkill
		s.PopupOpen = false
		return
	}
	commands := s.visibleCommandItems()
	if s.PaletteIndex < 0 || s.PaletteIndex >= len(commands) {
		return
	}
	s.PendingAction = commands[s.PaletteIndex].Action
	if s.PendingAction == PaletteActionNone {
		return
	}
	if s.PendingAction != PaletteActionSessionsList && s.PendingAction != PaletteActionSkillsList && s.PendingAction != PaletteActionChangeModel && s.PendingAction != PaletteActionOpenPalette {
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
	s.PaletteQuery = ""
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

func (s *State) SetModelItems(items []ModelListItem) {
	s.ModelItems = items
	s.PaletteView = PaletteViewModels
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	if len(items) > 0 {
		for i, item := range items {
			if item.ProviderID == s.ModelProvider && item.ModelID == s.ModelID {
				s.PaletteIndex = i
				break
			}
		}
	}
	s.PopupOpen = true
}

func (s *State) SetSkillItems(items []SkillListItem) {
	s.SkillItems = items
	s.PaletteView = PaletteViewSkills
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	s.PopupOpen = true
}

func (s *State) SetAvailableSkills(items []SkillListItem) {
	s.SkillItems = items
}

func (s *State) SelectedModel() (providerID, modelID string) {
	models := s.visibleModelItems()
	if s.PaletteView != PaletteViewModels || s.PaletteIndex < 0 || s.PaletteIndex >= len(models) {
		return "", ""
	}
	item := models[s.PaletteIndex]
	return item.ProviderID, item.ModelID
}

func (s *State) SelectedSessionID() string {
	sessions := s.visibleSessionItems()
	if s.PaletteView != PaletteViewSessions || s.PaletteIndex < 0 || s.PaletteIndex >= len(sessions) {
		return ""
	}
	return sessions[s.PaletteIndex].ID
}

func (s *State) SelectedSkill() (SkillListItem, bool) {
	skills := s.visibleSkillItems()
	if s.PaletteView != PaletteViewSkills || s.PaletteIndex < 0 || s.PaletteIndex >= len(skills) {
		return SkillListItem{}, false
	}
	return skills[s.PaletteIndex], true
}

func (s *State) InsertSkillMarker(skill SkillListItem) {
	marker := "/skill " + skill.Name
	content := s.Editor.GetContent()
	if strings.TrimSpace(content) != "" {
		marker += "\n"
	}
	for _, ch := range marker {
		s.Editor.InsertRune(ch)
	}
}

func (s *State) commandItems() []paletteCommandItem {
	return s.commandConfig.commandItems(s)
}

func (s *State) visibleCommandItems() []paletteCommandItem {
	commands := s.commandItems()
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return commands
	}
	items := make([]paletteCommandItem, 0, len(commands))
	for _, command := range commands {
		if paletteMatches(query, command.Title, command.Detail) {
			items = append(items, command)
		}
	}
	return items
}

func (s *State) visibleModelItems() []ModelListItem {
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.ModelItems
	}
	items := make([]ModelListItem, 0, len(s.ModelItems))
	for _, model := range s.ModelItems {
		if paletteMatches(query, model.ModelName, model.ModelID, model.ProviderID, model.ProviderName) {
			items = append(items, model)
		}
	}
	return items
}

func (s *State) visibleSessionItems() []SessionListItem {
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.SessionItems
	}
	items := make([]SessionListItem, 0, len(s.SessionItems))
	for _, session := range s.SessionItems {
		if paletteMatches(query, session.Title, session.ID) {
			items = append(items, session)
		}
	}
	return items
}

func (s *State) visibleSkillItems() []SkillListItem {
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.SkillItems
	}
	items := make([]SkillListItem, 0, len(s.SkillItems))
	for _, skill := range s.SkillItems {
		if paletteMatches(query, skill.Name, skill.Description) {
			items = append(items, skill)
		}
	}
	return items
}

func normalizedQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func paletteMatches(query string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func (s *State) SetModel(providerID, modelID string) {
	s.ModelProvider = providerID
	s.ModelID = modelID
}

func (s *State) ClearConversation() {
	s.messages = nil
	s.aioutput = ""
	s.outputScroll = 0
	s.invalidateOutputMax()
}

func (s *State) SetConversation(messages []Message) {
	s.messages = messages
	s.aioutput = ""
	s.outputScroll = 0
	s.invalidateOutputMax()
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
	case "skills":
		s.Editor.SetContent("")
		s.PendingAction = PaletteActionSkillsList
		return true
	default:
		for _, slashCommand := range s.SlashCommands() {
			if strings.TrimPrefix(slashCommand.Name, "/") != command {
				continue
			}
			s.Editor.SetContent("")
			if slashCommand.Action == PaletteActionOpenPalette {
				s.OpenPalette()
				return true
			}
			if slashCommand.Action == PaletteActionNone {
				return true
			}
			s.PendingAction = slashCommand.Action
			return true
		}
		return false
	}
}
