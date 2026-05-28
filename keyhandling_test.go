package main

import (
	"testing"

	"plums/internal/app"
	"plums/internal/keyboard"
	"plums/internal/screen"
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

func TestCtrlCCopiesEditorSelection(t *testing.T) {
	state := app.NewState(80, 24)
	state.Editor.SetContent("copy")
	state.Editor.SelectLeft()

	var copied string
	oldWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { writeClipboard = oldWriteClipboard })

	handled, quit := handleKey(state, keyboard.Event{Type: keyboard.KeyCtrlC})
	if !handled || quit {
		t.Fatalf("expected Ctrl+C to copy without quit, handled=%v quit=%v", handled, quit)
	}
	if copied != "y" {
		t.Fatalf("expected selected text copied, got %q", copied)
	}
}

func TestCtrlZUndoesEditorChange(t *testing.T) {
	state := app.NewState(80, 24)
	state.Editor.SetContent("a")
	state.Editor.InsertRune('b')

	handled, quit := handleKey(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'z', Ctrl: true})
	if !handled || quit {
		t.Fatalf("expected Ctrl+Z undo without quit, handled=%v quit=%v", handled, quit)
	}
	if got := state.Editor.GetContent(); got != "a" {
		t.Fatalf("expected undo to restore content, got %q", got)
	}
}

func TestCtrlZUndoesPastedTextAsOneChange(t *testing.T) {
	state := app.NewState(80, 24)
	state.Editor.SetContent("before ")

	handled, quit := handleKey(state, keyboard.Event{Type: keyboard.KeyPaste, Text: "one\ntwo"})
	if !handled || quit {
		t.Fatalf("expected paste handled without quit, handled=%v quit=%v", handled, quit)
	}
	if got := state.Editor.GetContent(); got != "before one\ntwo" {
		t.Fatalf("expected pasted text inserted, got %q", got)
	}

	handled, quit = handleKey(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'z', Ctrl: true})
	if !handled || quit {
		t.Fatalf("expected Ctrl+Z undo without quit, handled=%v quit=%v", handled, quit)
	}
	if got := state.Editor.GetContent(); got != "before " {
		t.Fatalf("expected one undo to remove full paste, got %q", got)
	}
}

func TestMouseDragCopiesOutputSelection(t *testing.T) {
	state := app.NewState(80, 24)
	state.Layout = app.LayoutDefault
	state.AppendAiOutput("hello world")

	root := app.CreateDefaultLayout(state)
	root.Layout(0, 0, 80, 24)
	root.Render(screen.NewScreen(80, 24))

	var copied string
	oldWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { writeClipboard = oldWriteClipboard })

	for _, ev := range []keyboard.Event{
		{Type: keyboard.KeyMouseLeftDown, Mouse: true, MouseX: 2, MouseY: 1},
		{Type: keyboard.KeyMouseLeftDrag, Mouse: true, MouseX: 7, MouseY: 1},
		{Type: keyboard.KeyMouseLeftUp, Mouse: true, MouseX: 7, MouseY: 1},
	} {
		handled, quit := handleKey(state, ev)
		if !handled || quit {
			t.Fatalf("expected mouse event handled without quit, event=%#v handled=%v quit=%v", ev, handled, quit)
		}
	}
	if copied != "hello" {
		t.Fatalf("expected output selection copied, got %q", copied)
	}
}

func TestOutputMouseSelectionCopiesWhenReleasedOutsideOutput(t *testing.T) {
	state := app.NewState(80, 24)
	state.Layout = app.LayoutDefault
	state.AppendAiOutput("hello world")

	root := app.CreateDefaultLayout(state)
	root.Layout(0, 0, 80, 24)
	root.Render(screen.NewScreen(80, 24))

	var copied string
	oldWriteClipboard := writeClipboard
	writeClipboard = func(text string) error {
		copied = text
		return nil
	}
	t.Cleanup(func() { writeClipboard = oldWriteClipboard })

	for _, ev := range []keyboard.Event{
		{Type: keyboard.KeyMouseLeftDown, Mouse: true, MouseX: 2, MouseY: 1},
		{Type: keyboard.KeyMouseLeftDrag, Mouse: true, MouseX: 7, MouseY: 20},
		{Type: keyboard.KeyMouseLeftUp, Mouse: true, MouseX: 7, MouseY: 20},
	} {
		handled, quit := handleKey(state, ev)
		if !handled || quit {
			t.Fatalf("expected mouse event handled without quit, event=%#v handled=%v quit=%v", ev, handled, quit)
		}
	}
	if copied != "hello world" {
		t.Fatalf("expected output selection copied after outside release, got %q", copied)
	}
}

func TestMouseDragSelectsEditorTextThroughKeyHandling(t *testing.T) {
	state := app.NewState(80, 24)
	state.Layout = app.LayoutDefault
	state.Editor.SetContent("hello world")

	root := app.CreateDefaultLayout(state)
	root.Layout(0, 0, 80, 24)
	root.Render(screen.NewScreen(80, 24))

	for _, ev := range []keyboard.Event{
		{Type: keyboard.KeyMouseLeftDown, Mouse: true, MouseX: 6, MouseY: 20},
		{Type: keyboard.KeyMouseLeftDrag, Mouse: true, MouseX: 11, MouseY: 20},
		{Type: keyboard.KeyMouseLeftUp, Mouse: true, MouseX: 11, MouseY: 20},
	} {
		handled, quit := handleKey(state, ev)
		if !handled || quit {
			t.Fatalf("expected mouse event handled without quit, event=%#v handled=%v quit=%v", ev, handled, quit)
		}
	}
	if got := state.Editor.SelectedText(); got != "hello" {
		t.Fatalf("expected editor mouse selection, got %q", got)
	}
}
