package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

func TestClampOutputScroll(t *testing.T) {
	state := NewState(80, 24)
	state.ScrollOutput(100)
	state.ClampOutputScroll(10)
	if got := state.OutputScroll(); got != 10 {
		t.Fatalf("expected scroll offset 10, got %d", got)
	}

	state.ClampOutputScroll(-1)
	if got := state.OutputScroll(); got != 0 {
		t.Fatalf("expected scroll offset 0, got %d", got)
	}
}

func TestResizeClampsInvalidDimensions(t *testing.T) {
	state := NewState(0, -1)
	if state.width != 1 || state.height != 1 {
		t.Fatalf("expected initial dimensions clamped to 1x1, got %dx%d", state.width, state.height)
	}

	state.Resize(-10, 0)
	if state.width != 1 || state.height != 1 {
		t.Fatalf("expected resized dimensions clamped to 1x1, got %dx%d", state.width, state.height)
	}
}

func TestScrollOutputVisibleUsesRenderedMax(t *testing.T) {
	state := NewState(80, 24)
	state.SetOutputMaxScroll(10)

	state.ScrollOutputVisible(100)
	if got := state.OutputScroll(); got != 10 {
		t.Fatalf("expected scroll offset 10, got %d", got)
	}

	state.AddMessage("ai", "new content invalidates exact max")
	state.ScrollOutputVisible(5)
	if got := state.OutputScroll(); got != 10 {
		t.Fatalf("expected scroll to keep using last rendered max while exact max is invalid, got %d", got)
	}
}

func TestLoadRenderConfig(t *testing.T) {
	if _, err := LoadRenderConfig(""); err != nil {
		t.Fatalf("load built-in render config: %v", err)
	}
	if _, err := LoadRenderConfig("testdata/layout.json"); err != nil {
		t.Fatalf("load test render config: %v", err)
	}
}

func TestSwitchLayoutSkipsUnavailableConfiguredLayouts(t *testing.T) {
	state := NewState(120, 40)
	cfg := &RenderConfig{Layouts: map[string]LayoutNode{
		"split": {},
		"chat":  {},
	}}
	state.SetAvailableLayouts(cfg.AvailableLayoutTypes())

	if state.Layout != LayoutSplit {
		t.Fatalf("expected initial layout to remain split, got %v", state.Layout)
	}
	state.SwitchLayout()
	if state.Layout != LayoutChat {
		t.Fatalf("expected tab to switch to chat, got %v", state.Layout)
	}
	state.SwitchLayout()
	if state.Layout != LayoutSplit {
		t.Fatalf("expected switch layout to skip missing default and return to split, got %v", state.Layout)
	}
}

func TestDefaultCommandConfigIncludesLayoutCommand(t *testing.T) {
	state := NewState(120, 40)
	state.OpenPalette()
	for _, ch := range "layout" {
		state.InsertPaletteRune(ch)
	}

	items := state.PaletteItems()
	if len(items) == 0 || items[0].Title != "Layouts" {
		t.Fatalf("expected layout command, got %#v", items)
	}
	if !strings.Contains(items[0].Detail, "split") {
		t.Fatalf("expected current layout in detail, got %q", items[0].Detail)
	}

	state.SelectPaletteItem()
	if got := state.ConsumePendingAction(); got != PaletteActionLayoutsList {
		t.Fatalf("expected layouts list action, got %v", got)
	}
}

func TestDefaultCommandConfigOmitsChatLayoutCommand(t *testing.T) {
	state := NewState(120, 40)
	state.OpenPalette()
	for _, ch := range "chat layout" {
		state.InsertPaletteRune(ch)
	}

	// Layout switching lives solely in the Layouts selector now; there is no
	// standalone "Chat layout" command in the palette.
	for _, item := range state.PaletteItems() {
		if item.Title == "Chat layout" {
			t.Fatalf("did not expect a standalone Chat layout command, got %#v", state.PaletteItems())
		}
	}
}

func TestNonSplitCommandPaletteHidesOutputPercentage(t *testing.T) {
	state := NewState(120, 40)
	state.Layout = LayoutZen
	state.OpenPalette()

	for _, item := range state.PaletteItems() {
		if item.Title == "Output percentage" {
			t.Fatalf("expected non-split palette to hide output percentage")
		}
	}
}

func TestLayoutPaletteSelection(t *testing.T) {
	state := NewState(120, 40)
	state.SetAvailableLayouts([]LayoutType{LayoutDefault, LayoutSplit, LayoutZen})
	state.SetLayoutItems()

	if state.PaletteTitle() != "Layouts" {
		t.Fatalf("expected layouts title, got %q", state.PaletteTitle())
	}
	items := state.PaletteItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 layout items, got %#v", items)
	}
	if items[1].Title != "Split" || !strings.Contains(items[1].Detail, "current") {
		t.Fatalf("expected split current item, got %#v", items[1])
	}

	state.MovePalette(1)
	state.SelectPaletteItem()
	if got := state.ConsumePendingAction(); got != PaletteActionSelectLayout {
		t.Fatalf("expected select layout action, got %v", got)
	}
	layoutType, ok := state.SelectedLayout()
	if !ok || layoutType != LayoutZen {
		t.Fatalf("expected zen selection, got %v ok=%v", layoutType, ok)
	}
	state.SetLayout(layoutType)
	if state.Layout != LayoutZen {
		t.Fatalf("expected zen layout, got %v", state.Layout)
	}
}

func TestLoadCommandConfig(t *testing.T) {
	if _, err := LoadCommandConfig(""); err != nil {
		t.Fatalf("load built-in command config: %v", err)
	}
	if _, err := LoadCommandConfig("testdata/commands.json"); err != nil {
		t.Fatalf("load test command config: %v", err)
	}
}

func TestCommandConfigControlsSlashCommands(t *testing.T) {
	cfg, err := LoadCommandConfig("testdata/commands.json")
	if err != nil {
		t.Fatalf("load command config: %v", err)
	}
	state := NewState(80, 24)
	state.SetCommandConfig(cfg)
	state.Editor.SetContent("/command")
	state.SubmitInput()

	if !state.PopupOpen {
		t.Fatalf("expected configured /command slash command to open palette")
	}
}

func TestDefaultCommandConfigIncludesSkillsCommand(t *testing.T) {
	state := NewState(80, 24)
	state.Editor.SetContent("/skills")
	state.SubmitInput()

	if got := state.ConsumePendingAction(); got != PaletteActionSkillsList {
		t.Fatalf("expected /skills to open skills list, got %v", got)
	}
}

func TestDefaultCommandConfigIncludesBackendCommand(t *testing.T) {
	state := NewState(80, 24)
	state.Editor.SetContent("/backend")
	state.SubmitInput()

	if got := state.ConsumePendingAction(); got != PaletteActionBackendList {
		t.Fatalf("expected /backend to open backend list, got %v", got)
	}
}

func TestRegistryCommandsControlSlashCommands(t *testing.T) {
	state := NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{Name: "/command", Detail: "Open from registry"},
	})
	state.Editor.SetContent("/command")
	state.SubmitInput()

	if !state.PopupOpen {
		t.Fatalf("expected registry /command slash command to open palette")
	}

	state = NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{Name: "/backend", Detail: "Switch backend"},
	})
	state.Editor.SetContent("/skills")
	state.SubmitInput()
	if got := state.ConsumePendingAction(); got != PaletteActionNone {
		t.Fatalf("expected /skills to be disabled by registry command set, got %v", got)
	}
}

func TestRegistrySlashCommandDropdownUsesRegistryDetails(t *testing.T) {
	state := NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{Name: "/custom", Detail: "Custom registry command"},
	})
	state.Editor.SetContent("/cu")

	commands := state.SlashCommands()
	if len(commands) != 1 || commands[0].Name != "/custom" || commands[0].Detail != "Custom registry command" {
		t.Fatalf("slash commands = %#v", commands)
	}
}

func TestRegistrySlashCommandWithDoSetsPendingCommand(t *testing.T) {
	state := NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{
			Name: "/custom",
			Do: func(context.Context, capabilities.Ctx) error {
				return nil
			},
		},
	})
	state.Editor.SetContent("/custom")
	state.SubmitInput()

	command, ok := state.ConsumePendingCommand()
	if !ok || command.Name != "/custom" || command.Do == nil {
		t.Fatalf("pending command = %+v ok=%v", command, ok)
	}
}

func TestRegistryCommandWithDoAppearsInPalette(t *testing.T) {
	state := NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{
			Name:   "/custom",
			Detail: "Custom registry command",
			Do: func(context.Context, capabilities.Ctx) error {
				return nil
			},
		},
	})
	state.OpenPalette()
	state.PaletteQuery = "custom"
	state.SelectPaletteItem()

	if got := state.ConsumePendingAction(); got != PaletteActionNone {
		t.Fatalf("pending action = %v, want none", got)
	}
	command, ok := state.ConsumePendingCommand()
	if !ok {
		t.Fatal("expected pending registry command")
	}
	if command.Name != "/custom" {
		t.Fatalf("pending command = %+v", command)
	}
	if state.PopupOpen {
		t.Fatal("palette should close after selecting registry command")
	}
}

func TestBackendPaletteSelection(t *testing.T) {
	state := NewState(80, 24)
	state.SetBackendProvider("opencode")
	state.SetBackendItems([]BackendListItem{
		{ID: "opencode", Name: "Opencode", Current: true},
		{ID: "codex", Name: "Codex"},
	})
	state.MovePalette(1)
	state.SelectPaletteItem()

	if got := state.ConsumePendingAction(); got != PaletteActionSelectBackend {
		t.Fatalf("expected select backend action, got %v", got)
	}
	if got := state.SelectedBackendID(); got != "codex" {
		t.Fatalf("expected codex backend selection, got %q", got)
	}
}

func TestDefaultCommandConfigIncludesThinkingVisibilityCommand(t *testing.T) {
	state := NewState(80, 24)
	state.OpenPalette()
	for _, ch := range "thinking" {
		state.InsertPaletteRune(ch)
	}

	items := state.PaletteItems()
	if len(items) != 1 || items[0].Title != "Thinking visibility" {
		t.Fatalf("expected thinking visibility command, got %#v", items)
	}
	if !strings.Contains(items[0].Detail, "full") {
		t.Fatalf("expected current visibility in detail, got %q", items[0].Detail)
	}

	state.SelectPaletteItem()
	if got := state.ConsumePendingAction(); got != PaletteActionCycleThinkingVisibility {
		t.Fatalf("expected thinking visibility action, got %v", got)
	}
}

func TestCycleThinkingVisibilityUpdatesChatLog(t *testing.T) {
	state := NewState(80, 24)
	if state.ThinkingMode != components.ThinkingVisibilityHidden {
		t.Fatalf("expected hidden visibility by default, got %v", state.ThinkingMode)
	}
	state.CycleThinkingVisibility()
	if state.ThinkingMode != components.ThinkingVisibilityFull {
		t.Fatalf("expected full visibility, got %v", state.ThinkingMode)
	}
	state.CycleThinkingVisibility()
	if state.ThinkingMode != components.ThinkingVisibilityTitle {
		t.Fatalf("expected title visibility, got %v", state.ThinkingMode)
	}
	state.CycleThinkingVisibility()
	if state.ThinkingMode != components.ThinkingVisibilityHidden {
		t.Fatalf("expected hidden visibility, got %v", state.ThinkingMode)
	}
}

func TestEditorSkillsCommandDoesNotRequireCommandConfig(t *testing.T) {
	state := NewState(80, 24)
	state.SetCommandConfig(&CommandConfig{
		Version: 1,
		SlashCommands: []SlashCommandConfig{
			{Name: "/command", Detail: "Open palette", Action: "open_palette"},
		},
		Palette: PaletteConfig{Items: []PaletteItemConfig{
			{Title: "Open palette", Action: "open_palette"},
		}},
		Actions: map[string]ActionSpec{
			"open_palette": {Kind: "builtin"},
		},
	})
	state.Editor.SetContent("/skills")
	state.SubmitInput()

	if got := state.ConsumePendingAction(); got != PaletteActionSkillsList {
		t.Fatalf("expected built-in /skills action, got %v", got)
	}
}

func TestSkillMarkerExpandsSubmittedPrompt(t *testing.T) {
	state := NewState(80, 24)
	state.SetSkillItems([]SkillListItem{{Name: "demo-skill", Content: "## Instructions\nUse demo behavior."}})
	state.ClosePalette()
	state.Editor.SetContent("/skill demo-skill\ndo the work")
	state.SubmitInput()

	got := state.ConsumeSubmittedInput()
	for _, want := range []string{
		"Use the `demo-skill` skill",
		"<skill_content name=\"demo-skill\">",
		"Use demo behavior.",
		"User request:\ndo the work",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected submitted input to contain %q, got %q", want, got)
		}
	}
}

func TestSubmitPromptUsesSameSubmissionStateAsEditorInput(t *testing.T) {
	state := NewState(80, 24)
	state.SetSkillItems([]SkillListItem{{Name: "demo-skill", Content: "Use demo behavior."}})
	state.Editor.SetContent("draft")

	state.SubmitPrompt("/skill demo-skill\ndo the work")

	messages := state.Messages()
	if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != "/skill demo-skill\ndo the work" {
		t.Fatalf("messages = %+v", messages)
	}
	if got := state.Editor.GetContent(); got != "" {
		t.Fatalf("editor content = %q, want cleared", got)
	}
	if got := state.ConsumeSubmittedInput(); !strings.Contains(got, "Use demo behavior.") {
		t.Fatalf("submitted input = %q, want expanded skill content", got)
	}
	if got := state.ConsumeSubmittedMessage(); got != "/skill demo-skill\ndo the work" {
		t.Fatalf("submitted message = %q, want raw prompt", got)
	}
}

func TestFinalizeAiOutputReturnsCommittedMessage(t *testing.T) {
	state := NewState(80, 24)
	state.AppendAiOutput("answer")

	if got := state.FinalizeAiOutput(); got != "answer" {
		t.Fatalf("FinalizeAiOutput() = %q, want answer", got)
	}
	messages := state.Messages()
	if len(messages) != 1 || messages[0].Role != "ai" || messages[0].Content != "answer" {
		t.Fatalf("messages = %+v", messages)
	}
	if got := state.FinalizeAiOutput(); got != "" {
		t.Fatalf("second FinalizeAiOutput() = %q, want empty", got)
	}
}

func TestInsertSkillMarkerLeavesEditorEditable(t *testing.T) {
	state := NewState(80, 24)
	state.Editor.SetContent("do the work")
	state.Editor.MoveCursorHome()
	state.InsertSkillMarker(SkillListItem{Name: "demo-skill"})

	if got := state.Editor.GetContent(); got != "/skill demo-skill\ndo the work" {
		t.Fatalf("expected skill marker in editor, got %q", got)
	}
	if len(state.Messages()) != 0 {
		t.Fatalf("expected no system message when inserting skill marker")
	}
}

func TestSkillSuggestionsAfterSkillDirective(t *testing.T) {
	state := NewState(80, 24)
	state.SetAvailableSkills([]SkillListItem{
		{Name: "frontend-design", Description: "Create polished UI"},
		{Name: "diagnose", Description: "Debug failures"},
	})
	state.Editor.SetContent("/skill front")

	suggestions := state.SkillSuggestions()
	if len(suggestions) != 1 || suggestions[0].Name != "frontend-design" {
		t.Fatalf("expected frontend-design suggestion, got %#v", suggestions)
	}
}

func TestFileCommandSuggestionsAtFirstCharOrAfterWhitespace(t *testing.T) {
	state := NewState(80, 24)
	state.projectFiles = []string{"README.md", "internal/app/state.go"}

	state.Editor.SetContent("@READ")
	suggestions := state.FileCommandSuggestions()
	if len(suggestions) != 1 || suggestions[0].Path != "README.md" {
		t.Fatalf("expected @ at first char to suggest README.md, got %#v", suggestions)
	}

	state.Editor.SetContent("open @state")
	suggestions = state.FileCommandSuggestions()
	if len(suggestions) != 1 || suggestions[0].Path != "internal/app/state.go" {
		t.Fatalf("expected @ after whitespace to suggest state.go, got %#v", suggestions)
	}

	state.Editor.SetContent("email@example")
	if suggestions := state.FileCommandSuggestions(); len(suggestions) != 0 {
		t.Fatalf("expected @ without leading whitespace ignored, got %#v", suggestions)
	}
}

func TestDiscoverSkillsFindsOpenCodeCompatibleSkill(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", filepath.Join(dir, "home"))
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	cwd := filepath.Join(dir, "nested", "pkg")
	skillDir := filepath.Join(dir, ".agents", "skills", "demo-skill")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create cwd: %v", err)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	path := filepath.Join(skillDir, "SKILL.md")
	content := "---\nname: demo-skill\ndescription: Demonstrate skill loading\n---\n## Body\nFollow these instructions.\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	skills, err := DiscoverSkills(cwd)
	if err != nil {
		t.Fatalf("discover skills: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %#v", skills)
	}
	if skills[0].Name != "demo-skill" || skills[0].Description != "Demonstrate skill loading" || !strings.Contains(skills[0].Content, "Follow these instructions") {
		t.Fatalf("unexpected skill: %#v", skills[0])
	}
}

func TestPaletteSearchSelectsFilteredCommand(t *testing.T) {
	state := NewState(80, 24)
	state.OpenPalette()
	for _, ch := range "session" {
		state.InsertPaletteRune(ch)
	}

	items := state.PaletteItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 session commands, got %d", len(items))
	}
	if items[0].Title != "Start new session" {
		t.Fatalf("expected first filtered command to be Start new session, got %q", items[0].Title)
	}

	state.MovePalette(1)
	state.SelectPaletteItem()
	if got := state.ConsumePendingAction(); got != PaletteActionSessionsList {
		t.Fatalf("expected sessions list action, got %v", got)
	}
}

func TestPaletteSearchSelectsFilteredModel(t *testing.T) {
	state := NewState(80, 24)
	state.SetModelItems([]ModelListItem{
		{ProviderID: "anthropic", ProviderName: "Anthropic", ModelID: "claude-sonnet-4", ModelName: "Claude Sonnet 4"},
		{ProviderID: "openai", ProviderName: "OpenAI", ModelID: "gpt-5.5", ModelName: "GPT 5.5"},
	})
	for _, ch := range "gpt" {
		state.InsertPaletteRune(ch)
	}

	items := state.PaletteItems()
	if len(items) != 1 || items[0].Title != "GPT 5.5" {
		t.Fatalf("expected GPT model only, got %#v", items)
	}
	providerID, modelID := state.SelectedModel()
	if providerID != "openai" || modelID != "gpt-5.5" {
		t.Fatalf("expected openai/gpt-5.5, got %s/%s", providerID, modelID)
	}
}

func TestQuestionPaletteSelectsAnswer(t *testing.T) {
	state := NewState(80, 24)
	state.SetQuestionItems("Pick one", []QuestionOptionItem{
		{Label: "It works", Description: "Confirm the tool works"},
		{Label: "Try again", Description: "Ask another question"},
	})

	if !state.PopupOpen {
		t.Fatalf("expected question palette to open")
	}
	state.MovePalette(1)
	state.SelectPaletteItem()
	if got := state.ConsumePendingAction(); got != PaletteActionAnswerQuestion {
		t.Fatalf("expected answer question action, got %v", got)
	}
	answer, ok := state.SelectedQuestionAnswer()
	if !ok || answer != "Try again" {
		t.Fatalf("expected selected answer Try again, got %q ok=%v", answer, ok)
	}
}
