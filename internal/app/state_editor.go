package app

import (
	"context"
	"strings"

	"github.com/Ceinl/plums/capabilities"
)

type SlashCommand struct {
	Name   string
	Detail string
	Action PaletteAction
	Do     func(context.Context, capabilities.Ctx) error
}

type FileCommandSuggestion struct {
	Path string
}

type SkillSuggestion struct {
	Name        string
	Description string
}

type editorDropdownKind int

const (
	editorDropdownNone editorDropdownKind = iota
	editorDropdownSlash
	editorDropdownFile
	editorDropdownSkill
)

type editorDropdownItem struct {
	Value  string
	Detail string
}

func (s *State) SetCommandConfig(cfg *CommandConfig) {
	if cfg == nil {
		cfg = DefaultCommandConfig()
	}
	s.commandConfig = cfg
}

func (s *State) SetCommands(commands []capabilities.Command) {
	s.commands = append([]capabilities.Command(nil), commands...)
}

func (s *State) EditorDropdownOpen() bool {
	if s.DropdownHidden {
		return false
	}
	_, items := s.editorDropdownItems()
	return len(items) > 0
}

func (s *State) MoveEditorDropdown(delta int) bool {
	_, items := s.editorDropdownItems()
	if len(items) == 0 {
		s.DropdownIndex = 0
		return false
	}
	s.DropdownIndex = (s.DropdownIndex + delta + len(items)) % len(items)
	return true
}

func (s *State) SelectEditorDropdownItem() bool {
	kind, items := s.editorDropdownItems()
	if len(items) == 0 {
		s.DropdownIndex = 0
		return false
	}
	if s.DropdownIndex < 0 || s.DropdownIndex >= len(items) {
		s.DropdownIndex = 0
	}
	selected := items[s.DropdownIndex]
	s.DropdownIndex = 0
	s.DropdownHidden = false
	s.replaceActiveEditorDropdown(kind, selected.Value)
	return true
}

func (s *State) SubmitExactSlashCommand() bool {
	input := strings.TrimSpace(s.Editor.GetContent())
	if input == "" || strings.Contains(input, "\n") {
		return false
	}
	for _, command := range s.allSlashCommands() {
		if input == command.Name {
			return s.runSlashCommand(strings.TrimPrefix(command.Name, "/"))
		}
	}
	return false
}

func (s *State) CloseEditorDropdown() bool {
	if !s.EditorDropdownOpen() {
		return false
	}
	s.DropdownIndex = 0
	s.DropdownHidden = true
	return true
}

func (s *State) ShowEditorDropdown() {
	s.DropdownHidden = false
}

func (s *State) ActiveEditorDropdownIndex(total int) int {
	if total <= 0 {
		s.DropdownIndex = 0
		return 0
	}
	if s.DropdownIndex < 0 || s.DropdownIndex >= total {
		s.DropdownIndex = 0
	}
	return s.DropdownIndex
}

func (s *State) SlashCommands() []SlashCommand {
	_, ctx, ok := s.activeCommand("/")
	if !ok || strings.Contains(ctx.Query, "\n") {
		return nil
	}

	commands := s.allSlashCommands()
	// Gate which command names the registered '/' completion source surfaces, then
	// map them back to the richer SlashCommand records the dropdown needs.
	byName := make(map[string]SlashCommand, len(commands))
	for _, command := range commands {
		byName[command.Name] = command
	}
	items := make([]SlashCommand, 0, len(commands))
	for _, candidate := range s.completionCandidates('/', ctx.Query) {
		if command, found := byName[candidate.Value]; found {
			items = append(items, command)
		}
	}
	return items
}

// completionCandidates queries the registered completion source for the trigger,
// or returns nil when no registry/source is present (e.g. State built without a
// runtime). This routes the built-in dropdowns through the Completion capability.
func (s *State) completionCandidates(trigger rune, query string) []capabilities.Candidate {
	var source capabilities.CompletionSource
	if s.completion != nil {
		source = s.completion.source(trigger)
	}
	if source == nil {
		// Fall back to the built-in source bound to this state so a State without a
		// runtime registry (e.g. tests) still produces identical candidates.
		source = s.builtinCompletionSource(trigger)
	}
	if source == nil {
		return nil
	}
	return source.Candidates(query)
}

// builtinCompletionSource returns the default source for a trigger bound to this
// state. It mirrors buildCompletionRegistry's built-ins for the no-registry path.
func (s *State) builtinCompletionSource(trigger rune) capabilities.CompletionSource {
	switch trigger {
	case '@':
		return fileCompletionSource{paths: func() []string {
			if s.projectFiles == nil {
				s.projectFiles = projectFilePaths(".", 5000)
			}
			return s.projectFiles
		}}
	case '/':
		return slashCompletionSource{commands: s.allSlashCommands}
	default:
		return nil
	}
}

func (s *State) allSlashCommands() []SlashCommand {
	if len(s.commands) == 0 {
		items := make([]SlashCommand, 0, len(s.commandConfig.SlashCommands))
		for _, command := range s.commandConfig.SlashCommands {
			items = append(items, SlashCommand{Name: command.Name, Detail: command.Detail, Action: s.commandConfig.actionFor(command.Action)})
		}
		return items
	}
	items := make([]SlashCommand, 0, len(s.commands))
	for _, command := range s.commands {
		if command.Name == "" || !strings.HasPrefix(command.Name, "/") {
			continue
		}
		items = append(items, SlashCommand{Name: command.Name, Detail: command.Detail, Action: s.actionForSlashCommand(command.Name), Do: command.Do})
	}
	return items
}

func (s *State) actionForSlashCommand(name string) PaletteAction {
	for _, command := range s.commandConfig.SlashCommands {
		if command.Name == name {
			return s.commandConfig.actionFor(command.Action)
		}
	}
	return PaletteActionNone
}

func (s *State) editorDropdownItems() (editorDropdownKind, []editorDropdownItem) {
	if skills := s.SkillSuggestions(); len(skills) > 0 {
		items := make([]editorDropdownItem, len(skills))
		for i, skill := range skills {
			items[i] = editorDropdownItem{Value: skill.Name, Detail: skill.Description}
		}
		return editorDropdownSkill, items
	}
	if files := s.FileCommandSuggestions(); len(files) > 0 {
		items := make([]editorDropdownItem, len(files))
		for i, file := range files {
			items[i] = editorDropdownItem{Value: file.Path}
		}
		return editorDropdownFile, items
	}
	commands := s.SlashCommands()
	if len(commands) > 0 {
		items := make([]editorDropdownItem, len(commands))
		for i, command := range commands {
			items[i] = editorDropdownItem{Value: command.Name, Detail: command.Detail}
		}
		return editorDropdownSlash, items
	}
	return editorDropdownNone, nil
}

func (s *State) replaceActiveEditorDropdown(kind editorDropdownKind, value string) {
	switch kind {
	case editorDropdownSkill:
		s.Editor.SetContent("/skill " + value)
	case editorDropdownFile:
		_, ctx, ok := s.activeCommand("@")
		if !ok {
			return
		}
		s.replaceEditorRange(ctx.TriggerStart, ctx.CursorIndex, "@"+value)
	case editorDropdownSlash:
		_, ctx, ok := s.activeCommand("/")
		if !ok {
			return
		}
		s.replaceEditorRange(ctx.TriggerStart, ctx.CursorIndex, value)
	}
}

func (s *State) replaceEditorRange(start, end int, replacement string) {
	content := s.Editor.GetContent()
	if start < 0 || end < start || end > len(content) {
		return
	}
	s.Editor.SetContent(content[:start] + replacement + content[end:])
}

func (s *State) FileCommandSuggestions() []FileCommandSuggestion {
	_, ctx, ok := s.activeCommand("@")
	if !ok {
		return nil
	}
	candidates := s.completionCandidates('@', ctx.Query)
	items := make([]FileCommandSuggestion, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, FileCommandSuggestion{Path: candidate.Value})
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

func (s *State) runEditorCommand(input string) bool {
	line := strings.TrimSpace(input)
	if strings.HasPrefix(line, "/") {
		command, ctx, ok := s.activeCommand("/")
		if !ok || command.HandlerFunction == nil {
			return false
		}
		return command.HandlerFunction(s, ctx)
	}
	if !strings.HasPrefix(line, ">") {
		return false
	}
	command := strings.ToLower(strings.TrimSpace(line[1:]))
	return s.runSlashCommand(command)
}

func (s *State) runSlashCommand(command string) bool {
	command = strings.ToLower(strings.TrimSpace(command))
	switch command {
	case "clear":
		s.Editor.SetContent("")
		return true
	case "command":
		if len(s.commands) == 0 {
			s.Editor.SetContent("")
			s.OpenPalette()
			return true
		}
	case "skills":
		if len(s.commands) == 0 {
			s.Editor.SetContent("")
			s.PendingAction = PaletteActionSkillsList
			return true
		}
	default:
	}
	for _, slashCommand := range s.allSlashCommands() {
		if strings.TrimPrefix(slashCommand.Name, "/") != command {
			continue
		}
		s.Editor.SetContent("")
		if slashCommand.Do != nil {
			s.pendingCommand = slashCommand.Name
			return true
		}
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

func (s *State) ConsumePendingCommand() (capabilities.Command, bool) {
	name := s.pendingCommand
	s.pendingCommand = ""
	if name == "" {
		return capabilities.Command{}, false
	}
	for _, command := range s.commands {
		if command.Name == name {
			return command, true
		}
	}
	return capabilities.Command{}, false
}
