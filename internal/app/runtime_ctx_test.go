package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestRuntimeCtxQueuesStateMutations(t *testing.T) {
	state := NewState(80, 24)
	mutations := make(chan stateMutation, 3)
	ctx := newRuntimeCtx(state, RunConfig{WorkingDirectory: "/tmp/project"}, mutations, nil)

	ctx.Chat("system", "hello")
	ctx.SetLayout("zen")
	ctx.Input().SetText("next")

	for i := 0; i < 3; i++ {
		mutation := <-mutations
		mutation(state)
	}

	messages := state.Messages()
	if len(messages) != 1 || messages[0].Role != "system" || messages[0].Content != "hello" {
		t.Fatalf("messages = %+v", messages)
	}
	if state.Layout != LayoutZen {
		t.Fatalf("layout = %q, want zen", state.Layout)
	}
	if got := state.Editor.GetContent(); got != "next" {
		t.Fatalf("editor content = %q, want next", got)
	}
}

func TestRuntimeCtxCapturesStateSnapshot(t *testing.T) {
	state := NewState(80, 24)
	state.SetSessionID("s1")
	state.SetSessionTitle("Session")
	state.SetModel("provider", "model")
	state.Editor.SetContent("prompt")

	ctx := newRuntimeCtx(state, RunConfig{}, make(chan stateMutation, 1), nil)
	if session := ctx.Session(); session.ID != "s1" || session.Title != "Session" || session.Model == nil || session.Model.ID != "model" {
		t.Fatalf("session snapshot = %+v", session)
	}
	if got := ctx.Input().Text(); got != "prompt" {
		t.Fatalf("input snapshot = %q", got)
	}
}

func TestRuntimeCtxSendQueuesPrompt(t *testing.T) {
	state := NewState(80, 24)
	prompts := make(chan string, 1)
	ctx := newRuntimeCtx(state, RunConfig{}, nil, prompts)

	ctx.Send("  prompt  ")

	if got := <-prompts; got != "prompt" {
		t.Fatalf("prompt = %q, want prompt", got)
	}
	if got := state.Editor.GetContent(); got != "" {
		t.Fatalf("editor content = %q, want unchanged empty input", got)
	}
}

func TestRuntimeCtxOpenListQueuesInteractivePalette(t *testing.T) {
	state := NewState(80, 24)
	mutations := make(chan stateMutation, 1)
	var picked capabilities.ListItem
	ctx := newRuntimeCtx(state, RunConfig{}, mutations, nil)

	ctx.OpenList("Branches", []capabilities.ListItem{
		{ID: "main", Label: "main", Detail: "primary"},
		{ID: "feature", Label: "feature/api", Detail: "work"},
	}, func(item capabilities.ListItem) {
		picked = item
	})
	mutation := <-mutations
	mutation(state)

	if !state.PopupOpen || state.PaletteView != PaletteViewList {
		t.Fatalf("palette open/view = %v/%v, want runtime list", state.PopupOpen, state.PaletteView)
	}
	if got := state.PaletteTitle(); got != "Branches" {
		t.Fatalf("palette title = %q, want Branches", got)
	}
	items := state.PaletteItems()
	if len(items) != 2 || items[1].Title != "feature/api" {
		t.Fatalf("palette items = %+v", items)
	}

	state.InsertPaletteRune('f')
	state.SelectPaletteItem()
	item, onPick, ok := state.ConsumePendingListPick()
	if !ok {
		t.Fatal("expected pending list pick")
	}
	onPick(item)
	if picked.ID != "feature" {
		t.Fatalf("picked = %+v, want feature", picked)
	}
	if len(state.ListItems) != 0 {
		t.Fatalf("runtime list was not cleared: %+v", state.ListItems)
	}
}
