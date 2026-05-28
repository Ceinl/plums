package components

import "testing"

func TestEditorUndoRestoresDeletedSelection(t *testing.T) {
	ed := NewTextEditor()
	ed.SetContent("hello")
	ed.SelectLeft()
	ed.SelectLeft()
	ed.DeleteSelection()

	if got := ed.GetContent(); got != "hel" {
		t.Fatalf("expected selected text deleted, got %q", got)
	}
	if !ed.Undo() {
		t.Fatal("expected undo to restore deletion")
	}
	if got := ed.GetContent(); got != "hello" {
		t.Fatalf("expected undo to restore content, got %q", got)
	}
}

func TestEditorUndoRestoresWordDeleteCursor(t *testing.T) {
	ed := NewTextEditor()
	ed.SetContent("hello world")
	ed.DeleteWordBackward()

	if got := ed.GetContent(); got != "hello " {
		t.Fatalf("expected word deleted, got %q", got)
	}
	if !ed.Undo() {
		t.Fatal("expected undo to restore word delete")
	}
	if got := ed.GetContent(); got != "hello world" {
		t.Fatalf("expected undo to restore content, got %q", got)
	}
	if ed.Cursor.Pos.Row != 0 || ed.Cursor.Pos.Col != len("hello world") {
		t.Fatalf("expected cursor restored to end, got %#v", ed.Cursor.Pos)
	}
}
