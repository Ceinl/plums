package app

import "testing"

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

func TestScrollOutputVisibleUsesRenderedMax(t *testing.T) {
	state := NewState(80, 24)
	state.SetOutputMaxScroll(10)

	state.ScrollOutputVisible(100)
	if got := state.OutputScroll(); got != 10 {
		t.Fatalf("expected scroll offset 10, got %d", got)
	}

	state.AddMessage("ai", "new content invalidates cached max")
	state.ScrollOutputVisible(5)
	if got := state.OutputScroll(); got != 15 {
		t.Fatalf("expected unclamped scroll while max is invalid, got %d", got)
	}
}

func TestLoadRenderConfig(t *testing.T) {
	if _, err := LoadRenderConfig(""); err != nil {
		t.Fatalf("load built-in render config: %v", err)
	}
	if _, err := LoadRenderConfig("../../docs/config/layout.json"); err != nil {
		t.Fatalf("load docs render config: %v", err)
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
