package app

import (
	"strings"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

type PaletteView int

const (
	PaletteViewCommands PaletteView = iota
	PaletteViewModels
	PaletteViewSessions
	PaletteViewSkills
	PaletteViewQuestions
	PaletteViewBackends
	PaletteViewLayouts
	PaletteViewList
)

type paletteCommandItem struct {
	Title       string
	Detail      string
	CommandName string
	Adjust      bool
	Step        int
	Disabled    bool
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
	items := s.PaletteItems()
	if s.PaletteIndex >= len(items) {
		s.PaletteIndex = 0
	}
	if len(items) > 0 && items[s.PaletteIndex].Disabled {
		s.MovePalette(1)
	}
}

func (s *State) ClosePalette() {
	if s.PaletteView == PaletteViewList {
		s.clearRuntimeList()
	}
	s.PopupOpen = false
	s.PaletteView = PaletteViewCommands
	s.PaletteQuery = ""
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
	if s.PaletteView == PaletteViewQuestions {
		if s.QuestionTitle != "" {
			return s.QuestionTitle
		}
		return "Question"
	}
	if s.PaletteView == PaletteViewBackends {
		return "Backend Providers"
	}
	if s.PaletteView == PaletteViewLayouts {
		return "Layouts"
	}
	if s.PaletteView == PaletteViewList {
		if s.ListTitle != "" {
			return s.ListTitle
		}
		return "List"
	}
	return paletteCommandsTitle
}

const (
	paletteCommandsTitle      = "Command Palette"
	paletteEmptySessionsTitle = "No sessions"
	paletteEmptySessionsHint  = "No opencode sessions found"
)

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
			return []components.PopupItem{{Title: paletteEmptySessionsTitle, Detail: paletteEmptySessionsHint, Disabled: true}}
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
	if s.PaletteView == PaletteViewQuestions {
		options := s.visibleQuestionItems()
		if len(options) == 0 {
			return []components.PopupItem{{Title: "No options", Detail: "Type an answer in the editor", Disabled: true}}
		}
		items := make([]components.PopupItem, len(options))
		for i, option := range options {
			items[i] = components.PopupItem{Title: option.Label, Detail: option.Description}
		}
		return items
	}
	if s.PaletteView == PaletteViewBackends {
		backends := s.visibleBackendItems()
		if len(backends) == 0 {
			return []components.PopupItem{{Title: "No backends", Detail: "No backend providers configured", Disabled: true}}
		}
		items := make([]components.PopupItem, len(backends))
		for i, backend := range backends {
			detail := backend.ID
			if backend.Current {
				detail = "current - " + detail
			}
			items[i] = components.PopupItem{Title: backend.Name, Detail: detail}
		}
		return items
	}
	if s.PaletteView == PaletteViewLayouts {
		layouts := s.visibleLayoutItems()
		if len(layouts) == 0 {
			return []components.PopupItem{{Title: "No layouts", Detail: "No layouts configured", Disabled: true}}
		}
		items := make([]components.PopupItem, len(layouts))
		for i, layoutType := range layouts {
			detail := layoutLabel(layoutType)
			if layoutType == s.Layout {
				detail = "current - " + detail
			}
			items[i] = components.PopupItem{Title: layoutTitle(layoutType), Detail: detail}
		}
		return items
	}
	if s.PaletteView == PaletteViewList {
		listItems := s.visibleRuntimeListItems()
		if len(listItems) == 0 {
			return []components.PopupItem{{Title: "No items", Detail: "", Disabled: true}}
		}
		items := make([]components.PopupItem, len(listItems))
		for i, item := range listItems {
			title := item.Label
			if title == "" {
				title = item.ID
			}
			items[i] = components.PopupItem{Title: title, Detail: item.Detail}
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

func (s *State) AdjustSelectedPaletteItem(delta int) bool {
	command, ok := s.selectedAdjustableCommand()
	if !ok {
		return false
	}
	step := command.Step
	if step == 0 {
		step = outputPercentageStep
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
	s.SelectPaletteItemWithCtx(nil)
}

func (s *State) SelectPaletteItemWithCtx(ctx capabilities.Ctx) {
	items := s.PaletteItems()
	if s.PaletteIndex < 0 || s.PaletteIndex >= len(items) || items[s.PaletteIndex].Disabled {
		return
	}
	if s.PaletteView == PaletteViewSessions {
		sessionID := s.SelectedSessionID()
		s.PopupOpen = false
		if ctx != nil {
			ctx.Sessions().Open(sessionID)
		}
		return
	}
	if s.PaletteView == PaletteViewModels {
		providerID, modelID := s.SelectedModel()
		s.PopupOpen = false
		if ctx != nil {
			ctx.Backends().SetModel(providerID, modelID)
		}
		return
	}
	if s.PaletteView == PaletteViewSkills {
		if skill, ok := s.SelectedSkill(); ok {
			s.InsertSkillMarker(skill)
		}
		s.PopupOpen = false
		return
	}
	if s.PaletteView == PaletteViewQuestions {
		answer, ok := s.SelectedQuestionAnswer()
		s.PopupOpen = false
		if ok && ctx != nil {
			ctx.Backends().AnswerQuestion(answer)
		}
		return
	}
	if s.PaletteView == PaletteViewBackends {
		backendID := s.SelectedBackendID()
		s.PopupOpen = false
		if ctx != nil {
			ctx.Backends().Select(backendID)
		}
		return
	}
	if s.PaletteView == PaletteViewLayouts {
		if layoutType, ok := s.SelectedLayout(); ok {
			s.SetLayout(layoutType)
		}
		s.PopupOpen = false
		return
	}
	if s.PaletteView == PaletteViewList {
		s.pendingListPick = true
		s.PopupOpen = false
		return
	}
	commands := s.visibleCommandItems()
	if s.PaletteIndex < 0 || s.PaletteIndex >= len(commands) {
		return
	}
	command := commands[s.PaletteIndex]
	if command.CommandName == "" {
		return
	}
	// The command's Do verb decides whether a new picker/list opens (model,
	// backend, sessions, skills, layouts) or the palette closes (toggles, new
	// session). For the picker-opening verbs the palette stays open until the run
	// loop swaps the view; the others close it here. Output-percent adjusts in
	// place via Left/Right and never selects.
	if !command.Adjust {
		s.pendingCommand = command.CommandName
		if !commandKeepsPaletteOpen(command.CommandName) {
			s.PopupOpen = false
		}
	}
}

// commandKeepsPaletteOpen reports whether selecting a command leaves the palette
// open (because the command swaps to a sub-view: models, backends, sessions,
// skills, layouts, or re-opens the palette), matching the legacy dispatch gate.
func commandKeepsPaletteOpen(name string) bool {
	switch name {
	case "Change model", "Backend provider", "Sessions list", "Skills list", "Layouts":
		return true
	default:
		return false
	}
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

func (s *State) SetSkillItems(items []capabilities.Skill) {
	s.SkillItems = items
	s.PaletteView = PaletteViewSkills
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	s.PopupOpen = true
}

func (s *State) SetQuestionItems(title string, items []capabilities.QuestionOption) {
	s.QuestionTitle = title
	s.QuestionItems = items
	s.PaletteView = PaletteViewQuestions
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	s.PopupOpen = true
	s.ensurePaletteSelection()
}

func (s *State) SetBackendItems(items []BackendListItem) {
	s.BackendItems = items
	s.PaletteView = PaletteViewBackends
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	if len(items) > 0 {
		for i, item := range items {
			if item.ID == s.BackendProvider {
				s.PaletteIndex = i
				break
			}
		}
	}
	s.PopupOpen = true
}

func (s *State) SetRuntimeList(title string, items []capabilities.ListItem, onPick func(capabilities.ListItem)) {
	s.ListTitle = title
	s.ListItems = append([]capabilities.ListItem(nil), items...)
	s.listOnPick = onPick
	s.PaletteView = PaletteViewList
	s.PaletteQuery = ""
	s.PaletteIndex = 0
	s.PopupOpen = true
	s.ensurePaletteSelection()
}

func (s *State) ConsumePendingListPick() (capabilities.ListItem, func(capabilities.ListItem), bool) {
	if !s.pendingListPick {
		return capabilities.ListItem{}, nil, false
	}
	s.pendingListPick = false
	items := s.visibleRuntimeListItems()
	if s.PaletteIndex < 0 || s.PaletteIndex >= len(items) {
		s.clearRuntimeList()
		return capabilities.ListItem{}, nil, false
	}
	item := items[s.PaletteIndex]
	onPick := s.listOnPick
	s.clearRuntimeList()
	return item, onPick, true
}

func (s *State) clearRuntimeList() {
	s.ListTitle = ""
	s.ListItems = nil
	s.listOnPick = nil
}

func (s *State) SetAvailableBackends(items []BackendListItem) {
	s.BackendItems = items
}

func (s *State) SetAvailableSkills(items []capabilities.Skill) {
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

func (s *State) SelectedSkill() (capabilities.Skill, bool) {
	skills := s.visibleSkillItems()
	if s.PaletteView != PaletteViewSkills || s.PaletteIndex < 0 || s.PaletteIndex >= len(skills) {
		return capabilities.Skill{}, false
	}
	return skills[s.PaletteIndex], true
}

func (s *State) SelectedQuestionAnswer() (string, bool) {
	items := s.visibleQuestionItems()
	if s.PaletteView != PaletteViewQuestions || s.PaletteIndex < 0 || s.PaletteIndex >= len(items) {
		return "", false
	}
	return items[s.PaletteIndex].Label, true
}

func (s *State) SelectedBackendID() string {
	backends := s.visibleBackendItems()
	if s.PaletteView != PaletteViewBackends || s.PaletteIndex < 0 || s.PaletteIndex >= len(backends) {
		return ""
	}
	return backends[s.PaletteIndex].ID
}

// commandItems builds the command-palette rows from the registered commands.
// Each row's visible label comes from the command's dynamic Title (fed the
// current host state) when present, else its static Name/Detail. Title-disabled
// rows (e.g. the slash commands) are skipped so the palette shows only the
// palette actions. The split-only output-percent adjuster is hidden outside the
// split layout, matching the legacy behavior.
func (s *State) commandItems() []paletteCommandItem {
	state := s.commandState()
	items := make([]paletteCommandItem, 0, len(s.commands))
	for _, command := range s.commands {
		if command.Do == nil {
			continue
		}
		title := command.Name
		detail := command.Detail
		disabled := false
		adjust := false
		step := 0
		if command.Title != nil {
			label := command.Title(state)
			if label.Disabled {
				continue
			}
			title = label.Title
			detail = label.Detail
			adjust = label.Adjust
			step = label.Step
		}
		if adjust && s.EffectiveLayout() != LayoutSplit {
			continue
		}
		items = append(items, paletteCommandItem{
			Title:       title,
			Detail:      detail,
			CommandName: command.Name,
			Adjust:      adjust,
			Step:        step,
			Disabled:    disabled,
		})
	}
	return items
}

// commandState snapshots the host fields commands read for dynamic palette
// titles, mirroring runtime_ctx's commandStateFromState.
func (s *State) commandState() capabilities.CommandState {
	return capabilities.CommandState{
		Mode:               s.Mode,
		Layout:             s.LayoutLabel(),
		ThinkingVisibility: s.ThinkingVisibilityLabel(),
		ToolCallVisibility: s.ToolCallVisibilityLabel(),
		OutputPercent:      s.SplitOutputPercent(),
		BackendProvider:    s.BackendProvider,
	}
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

func (s *State) visibleRuntimeListItems() []capabilities.ListItem {
	if s.PaletteQuery == "" {
		return s.ListItems
	}
	items := make([]capabilities.ListItem, 0, len(s.ListItems))
	for _, item := range s.ListItems {
		if paletteMatches(s.PaletteQuery, item.ID, item.Label, item.Detail) {
			items = append(items, item)
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

func (s *State) visibleSkillItems() []capabilities.Skill {
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.SkillItems
	}
	items := make([]capabilities.Skill, 0, len(s.SkillItems))
	for _, skill := range s.SkillItems {
		if paletteMatches(query, skill.Name, skill.Description) {
			items = append(items, skill)
		}
	}
	return items
}

func (s *State) visibleQuestionItems() []capabilities.QuestionOption {
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.QuestionItems
	}
	items := make([]capabilities.QuestionOption, 0, len(s.QuestionItems))
	for _, option := range s.QuestionItems {
		if paletteMatches(query, option.Label, option.Description) {
			items = append(items, option)
		}
	}
	return items
}

func (s *State) visibleBackendItems() []BackendListItem {
	query := normalizedQuery(s.PaletteQuery)
	if query == "" {
		return s.BackendItems
	}
	items := make([]BackendListItem, 0, len(s.BackendItems))
	for _, backend := range s.BackendItems {
		if paletteMatches(query, backend.Name, backend.ID) {
			items = append(items, backend)
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
