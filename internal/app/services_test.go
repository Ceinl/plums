package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
)

func TestCompletionRegistryLastWinsPerTrigger(t *testing.T) {
	registry := newCompletionRegistry()
	first := stubCompletionSource{trigger: '@', value: "first"}
	second := stubCompletionSource{trigger: '@', value: "second"}
	registry.Register(first)
	registry.Register(second)

	source := registry.source('@')
	if source == nil {
		t.Fatal("expected a source for '@'")
	}
	got := source.Candidates("")
	if len(got) != 1 || got[0].Value != "second" {
		t.Fatalf("most-recent registration should win, got %+v", got)
	}
	if registry.source('/') != nil {
		t.Fatal("unexpected source for unregistered trigger")
	}
}

func TestBuildCompletionRegistryRegistersBuiltinsAndContributed(t *testing.T) {
	state := NewState(80, 24)
	contributed := stubCompletionSource{trigger: '#', value: "tag"}
	registry := buildCompletionRegistry(state, []capabilities.CompletionSource{contributed})

	if registry.source('@') == nil {
		t.Fatal("built-in @file source missing")
	}
	if registry.source('/') == nil {
		t.Fatal("built-in /slash source missing")
	}
	if registry.source('#') == nil {
		t.Fatal("contributed source missing")
	}
}

func TestClipboardServiceCopyRoutesThroughWriteClipboard(t *testing.T) {
	original := writeClipboard
	defer func() { writeClipboard = original }()

	var gotText, gotCmd string
	writeClipboard = func(text, command string) error {
		gotText, gotCmd = text, command
		return nil
	}

	rt := &runtimeCtx{clipboardCmd: "mycopy"}
	if err := rt.Services().Clipboard().Copy("hello"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if gotText != "hello" || gotCmd != "mycopy" {
		t.Fatalf("writeClipboard got (%q,%q)", gotText, gotCmd)
	}
}

func TestPaletteServiceOpenQueuesRuntimeList(t *testing.T) {
	state := NewState(80, 24)
	mutations := make(chan stateMutation, 1)
	rt := newRuntimeCtx(state, RunConfig{}, mutations, nil)

	rt.Services().Palette().Open("Branches", []capabilities.ListItem{{ID: "main", Label: "main"}}, nil)
	(<-mutations)(state)

	if !state.PopupOpen || state.PaletteView != PaletteViewList {
		t.Fatalf("palette not opened as runtime list: open=%v view=%v", state.PopupOpen, state.PaletteView)
	}
	if state.PaletteTitle() != "Branches" {
		t.Fatalf("title = %q", state.PaletteTitle())
	}
}

func TestSelectionServiceCopyUsesCurrentSelection(t *testing.T) {
	original := writeClipboard
	defer func() { writeClipboard = original }()
	var gotText string
	writeClipboard = func(text, command string) error {
		gotText = text
		return nil
	}

	rt := &runtimeCtx{selection: "picked text"}
	sel := rt.Services().Selection()
	if sel.Current() != "picked text" {
		t.Fatalf("current = %q", sel.Current())
	}
	if err := sel.Copy(); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if gotText != "picked text" {
		t.Fatalf("clipboard got %q", gotText)
	}
}

type stubCompletionSource struct {
	trigger rune
	value   string
}

func (s stubCompletionSource) Trigger() rune { return s.trigger }
func (s stubCompletionSource) Candidates(string) []capabilities.Candidate {
	return []capabilities.Candidate{{Value: s.value}}
}
