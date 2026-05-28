package app

import (
	"os"
	"path/filepath"
	"plums/internal/components"
	"strings"
	"testing"
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
		"split":      {},
		"fullscreen": {},
	}}
	state.SetAvailableLayouts(cfg.AvailableLayoutTypes())

	if state.Layout != LayoutSplit {
		t.Fatalf("expected initial layout to remain split, got %v", state.Layout)
	}
	state.SwitchLayout()
	if state.Layout != LayoutFullscreen {
		t.Fatalf("expected tab to switch to fullscreen, got %v", state.Layout)
	}
	state.SwitchLayout()
	if state.Layout != LayoutSplit {
		t.Fatalf("expected tab to skip missing default and return to split, got %v", state.Layout)
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
	state.CycleThinkingVisibility()
	if state.ThinkingMode != components.ThinkingVisibilityTitle {
		t.Fatalf("expected title visibility, got %v", state.ThinkingMode)
	}
	state.CycleThinkingVisibility()
	if state.ThinkingMode != components.ThinkingVisibilityHidden {
		t.Fatalf("expected hidden visibility, got %v", state.ThinkingMode)
	}
	state.CycleThinkingVisibility()
	if state.ThinkingMode != components.ThinkingVisibilityFull {
		t.Fatalf("expected full visibility, got %v", state.ThinkingMode)
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
