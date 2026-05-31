package app

import (
	"testing"

	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

func TestSessionMouseDownQueuesNewSessionAction(t *testing.T) {
	state := NewState(80, 24)
	sessions := state.Sessions()
	sessions.SetItems([]components.SessionItem{{ID: "s1", Title: "One"}})
	sessions.Layout(0, 0, 30, 10)
	sessions.Render(screen.NewScreen(80, 24))

	if !state.SessionMouseDown(1, 0) {
		t.Fatalf("expected session mouse hit")
	}
	if got := state.ConsumePendingAction(); got != PaletteActionNewSession {
		t.Fatalf("expected new session action, got %v", got)
	}
}

func TestSessionMouseDownQueuesSelectSessionAction(t *testing.T) {
	state := NewState(80, 24)
	state.SetSessionID("s1")
	state.PopupOpen = false
	state.SetSessionItems([]SessionListItem{{ID: "s1", Title: "One", Current: true}, {ID: "s2", Title: "Two"}})
	state.PopupOpen = false
	sessions := state.Sessions()
	sessions.SetItems([]components.SessionItem{{ID: "s1", Title: "One", Current: true}, {ID: "s2", Title: "Two"}})
	sessions.Layout(0, 0, 30, 10)
	sessions.Render(screen.NewScreen(80, 24))

	if !state.SessionMouseDown(1, 3) {
		t.Fatalf("expected session mouse hit")
	}
	if got := state.ConsumePendingAction(); got != PaletteActionSelectSession {
		t.Fatalf("expected select session action, got %v", got)
	}
	if got := state.SelectedSessionID(); got != "s2" {
		t.Fatalf("expected selected session s2, got %q", got)
	}
}
