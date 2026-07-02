package app

import (
	"context"
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

func TestNewRenderConfig(t *testing.T) {
	cfg := NewRenderConfig()
	if cfg.Version != 1 {
		t.Fatalf("expected version 1, got %d", cfg.Version)
	}
	if cfg.Layouts == nil {
		t.Fatal("expected non-nil layouts map")
	}
	if _, ok := cfg.Overlays["slash_command_dropdown"]; !ok {
		t.Fatal("expected slash_command_dropdown overlay scaffold")
	}
	// Layouts come from the registry via InstallPublicLayout; the scaffold itself
	// ships none.
	if len(cfg.Layouts) != 0 {
		t.Fatalf("expected empty scaffold layouts, got %d", len(cfg.Layouts))
	}
}

func TestSwitchLayoutSkipsUnavailableConfiguredLayouts(t *testing.T) {
	state := NewState(120, 40)
	state.Layout = LayoutSplit
	cfg := &RenderConfig{Layouts: map[string]LayoutNode{
		"split": {},
		"zen":   {},
	}}
	state.SetAvailableLayouts(cfg.AvailableLayoutTypes())

	if state.Layout != LayoutSplit {
		t.Fatalf("expected initial layout to remain split, got %v", state.Layout)
	}
	state.SwitchLayout()
	if state.Layout != LayoutZen {
		t.Fatalf("expected tab to switch to zen, got %v", state.Layout)
	}
	state.SwitchLayout()
	if state.Layout != LayoutSplit {
		t.Fatalf("expected switch layout to skip missing default and return to split, got %v", state.Layout)
	}
}

func TestPaletteIncludesLayoutCommand(t *testing.T) {
	state := stateWithBuiltinCommands(120, 40)
	state.Layout = LayoutSplit
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
	if got := runPending(state); !got.called("SwitchLayout") {
		t.Fatalf("expected layouts command to call SwitchLayout, got %v", got.calls)
	}
}

func TestPaletteOmitsChatLayoutCommand(t *testing.T) {
	state := stateWithBuiltinCommands(120, 40)
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
	state := stateWithBuiltinCommands(120, 40)
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
	state.SetAvailableLayouts([]LayoutType{LayoutZen, LayoutSplit})
	state.SetLayout(LayoutSplit)
	state.SetLayoutItems()

	if state.PaletteTitle() != "Layouts" {
		t.Fatalf("expected layouts title, got %q", state.PaletteTitle())
	}
	items := state.PaletteItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 layout items, got %#v", items)
	}
	if items[1].Title != "Split" || !strings.Contains(items[1].Detail, "current") {
		t.Fatalf("expected split current item, got %#v", items[1])
	}

	state.MovePalette(1)
	state.SelectPaletteItem()
	if state.Layout != LayoutZen {
		t.Fatalf("expected zen layout, got %v", state.Layout)
	}
}

func TestSlashSkillsCallsOpenSkills(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
	state.Editor.SetContent("/skills")
	state.SubmitInput()

	if got := runPending(state); !got.called("OpenSkills") {
		t.Fatalf("expected /skills to call OpenSkills, got %v", got.calls)
	}
}

func TestSlashBackendCallsSwitchBackend(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
	state.Editor.SetContent("/backend")
	state.SubmitInput()

	if got := runPending(state); !got.called("SwitchBackend") {
		t.Fatalf("expected /backend to call SwitchBackend, got %v", got.calls)
	}
}

func TestRegistryCommandsControlSlashCommands(t *testing.T) {
	state := NewState(80, 24)
	called := false
	state.SetCommands([]capabilities.Command{
		{Name: "/command", Detail: "Open from registry", Do: func(context.Context, capabilities.Ctx) error {
			called = true
			return nil
		}},
	})
	state.Editor.SetContent("/command")
	state.SubmitInput()

	command, ok := state.ConsumePendingCommand()
	if !ok || command.Name != "/command" {
		t.Fatalf("expected registry /command to be pending, got %+v ok=%v", command, ok)
	}
	_ = command.Do(context.Background(), &fakeCtx{})
	if !called {
		t.Fatal("expected registry /command Do to run")
	}

	// A command set without /skills means /skills does nothing.
	state = NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{Name: "/backend", Detail: "Switch backend", Do: func(context.Context, capabilities.Ctx) error { return nil }},
	})
	state.Editor.SetContent("/skills")
	state.SubmitInput()
	if _, ok := state.ConsumePendingCommand(); ok {
		t.Fatal("expected /skills to be absent from registry command set")
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
	ctx := &fakeCtx{}
	state.SelectPaletteItemWithCtx(ctx)

	if !ctx.called("SelectBackend") {
		t.Fatalf("expected select backend call, got %v", ctx.calls)
	}
}

func TestPaletteIncludesThinkingVisibilityCommand(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
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
	if got := runPending(state); !got.called("CycleThinkingVisibility") {
		t.Fatalf("expected thinking visibility command to call CycleThinkingVisibility, got %v", got.calls)
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

func TestSkillMarkerSubmitsRawPromptForRuntimeExpansion(t *testing.T) {
	state := NewState(80, 24)
	state.SetSkillItems([]capabilities.Skill{{Name: "demo-skill", Content: "## Instructions\nUse demo behavior."}})
	state.ClosePalette()
	state.Editor.SetContent("/skill demo-skill\ndo the work")
	state.SubmitInput()

	got := state.ConsumeSubmittedInput()
	if got != "/skill demo-skill\ndo the work" {
		t.Fatalf("submitted input = %q, want raw prompt", got)
	}
}

func TestSubmitPromptUsesSameSubmissionStateAsEditorInput(t *testing.T) {
	state := NewState(80, 24)
	state.SetSkillItems([]capabilities.Skill{{Name: "demo-skill", Content: "Use demo behavior."}})
	state.Editor.SetContent("draft")

	state.SubmitPrompt("/skill demo-skill\ndo the work")

	messages := state.Messages()
	if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != "/skill demo-skill\ndo the work" {
		t.Fatalf("messages = %+v", messages)
	}
	if got := state.Editor.GetContent(); got != "" {
		t.Fatalf("editor content = %q, want cleared", got)
	}
	if got := state.ConsumeSubmittedInput(); got != "/skill demo-skill\ndo the work" {
		t.Fatalf("submitted input = %q, want raw prompt", got)
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
	state.InsertSkillMarker(capabilities.Skill{Name: "demo-skill"})

	if got := state.Editor.GetContent(); got != "/skill demo-skill\ndo the work" {
		t.Fatalf("expected skill marker in editor, got %q", got)
	}
	if len(state.Messages()) != 0 {
		t.Fatalf("expected no system message when inserting skill marker")
	}
}

func TestSkillSuggestionsAfterSkillDirective(t *testing.T) {
	state := NewState(80, 24)
	state.SetAvailableSkills([]capabilities.Skill{
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

func TestPaletteSearchSelectsFilteredCommand(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
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
	if got := runPending(state); !got.called("OpenSessions") {
		t.Fatalf("expected sessions list command to call OpenSessions, got %v", got.calls)
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
	state.SetQuestionItems("Pick one", []capabilities.QuestionOption{
		{Label: "It works", Description: "Confirm the tool works"},
		{Label: "Try again", Description: "Ask another question"},
	})

	if !state.PopupOpen {
		t.Fatalf("expected question palette to open")
	}
	state.MovePalette(1)
	ctx := &fakeCtx{}
	state.SelectPaletteItemWithCtx(ctx)
	answer, ok := state.SelectedQuestionAnswer()
	if !ok || answer != "Try again" {
		t.Fatalf("expected selected answer Try again, got %q ok=%v", answer, ok)
	}
	if !ctx.called("AnswerQuestion") {
		t.Fatalf("expected answer question call, got %v", ctx.calls)
	}
}
