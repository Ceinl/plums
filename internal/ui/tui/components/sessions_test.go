package components

import (
	"testing"

	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

func TestSessionsVerticalMouseHitsNewAndItems(t *testing.T) {
	sessions := NewSessions(SessionsVertical)
	sessions.SetItems([]SessionItem{{ID: "s1", Title: "One"}, {ID: "s2", Title: "Two", Current: true}})
	sessions.Layout(10, 5, 20, 8)
	sessions.Render(screen.NewScreen(40, 20))

	action, id, ok := sessions.MouseDown(11, 5)
	if !ok || action != SessionMouseNew || id != "" {
		t.Fatalf("expected new session hit, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(11, 8)
	if !ok || action != SessionMouseSelect || id != "s2" {
		t.Fatalf("expected second session hit, got ok=%v action=%v id=%q", ok, action, id)
	}
}

func TestSessionsHorizontalMouseHitsTabsAndRightPlus(t *testing.T) {
	sessions := NewSessions(SessionsHorizontal)
	sessions.SetItems([]SessionItem{{ID: "s1", Title: "One"}, {ID: "s2", Title: "Two", Current: true}})
	sessions.Layout(0, 2, 30, 1)
	sessions.Render(screen.NewScreen(40, 10))

	action, id, ok := sessions.MouseDown(1, 2)
	if !ok || action != SessionMouseSelect || id != "s1" {
		t.Fatalf("expected first tab hit, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(28, 2)
	if !ok || action != SessionMouseNew || id != "" {
		t.Fatalf("expected right plus hit, got ok=%v action=%v id=%q", ok, action, id)
	}
}
