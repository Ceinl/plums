package main

import (
	"testing"

	"plums/internal/app"
	"plums/internal/keyboard"
)

func TestShiftEnterSubmitsInput(t *testing.T) {
	state := app.NewState(80, 24)
	state.Editor.SetContent("send me")

	handled, quit := handleKey(state, keyboard.Event{Type: keyboard.KeyEnter, Shift: true})
	if !handled || quit {
		t.Fatalf("expected handled submit without quit, handled=%v quit=%v", handled, quit)
	}
	if got := state.ConsumeSubmittedInput(); got != "send me" {
		t.Fatalf("expected submitted input, got %q", got)
	}
}

func TestPlainEnterInsertsNewline(t *testing.T) {
	state := app.NewState(80, 24)
	state.Editor.SetContent("line")

	handled, quit := handleKey(state, keyboard.Event{Type: keyboard.KeyEnter})
	if !handled || quit {
		t.Fatalf("expected handled newline without quit, handled=%v quit=%v", handled, quit)
	}
	if got := state.Editor.GetContent(); got != "line\n" {
		t.Fatalf("expected newline insertion, got %q", got)
	}
}

func TestCtrlCQuitsWhenPopupOpen(t *testing.T) {
	state := app.NewState(80, 24)
	state.OpenPalette()

	handled, quit := handleKey(state, keyboard.Event{Type: keyboard.KeyCtrlC})
	if handled || !quit {
		t.Fatalf("expected Ctrl+C to quit through popup, handled=%v quit=%v", handled, quit)
	}
}
