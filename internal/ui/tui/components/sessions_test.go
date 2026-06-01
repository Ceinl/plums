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

	// Layout: s.y=5, s.h=8 -> rows 0..7 (screen y 5..12)
	// row 0: "+ New session" card
	// row 1: 1-line gap
	// row 2: s1 card
	// row 3: 1-line gap
	// row 4: s2 card

	action, id, ok := sessions.MouseDown(11, 5)
	if !ok || action != SessionMouseNew || id != "" {
		t.Fatalf("expected new session hit, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(11, 6)
	if ok {
		t.Fatalf("expected no hit on gap row, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(11, 7)
	if !ok || action != SessionMouseSelect || id != "s1" {
		t.Fatalf("expected first session hit, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(11, 8)
	if ok {
		t.Fatalf("expected no hit on gap row, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(11, 9)
	if !ok || action != SessionMouseSelect || id != "s2" {
		t.Fatalf("expected second session hit, got ok=%v action=%v id=%q", ok, action, id)
	}
}

func TestSessionsVerticalGroupsByDirectory(t *testing.T) {
	sessions := NewSessions(SessionsVertical)
	sessions.SetItems([]SessionItem{
		{ID: "s1", Title: "Alpha", Directory: "/projects/beta", Updated: 100},
		{ID: "s2", Title: "Gamma", Directory: "/projects/alpha", Updated: 200},
		{ID: "s3", Title: "Delta", Directory: "/projects/beta", Updated: 300},
	})
	sessions.Layout(0, 0, 30, 10)
	sessions.Render(screen.NewScreen(40, 20))

	// Layout: y=0, h=10 -> rows 0..9
	// row 0: "+ New session" card
	// row 1: 1-line gap
	// row 2: "alpha" header
	// row 3: "Gamma" (s2) card
	// row 4: 1-line gap
	// row 5: "beta" header
	// row 6: "Delta" (s3) card
	// row 7: 1-line gap
	// row 8: "Alpha" (s1) card

	action, id, ok := sessions.MouseDown(1, 2)
	if ok {
		t.Fatalf("expected no hit on directory header, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(1, 3)
	if !ok || action != SessionMouseSelect || id != "s2" {
		t.Fatalf("expected s2 hit, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(1, 4)
	if ok {
		t.Fatalf("expected no hit on gap row, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(1, 6)
	if !ok || action != SessionMouseSelect || id != "s3" {
		t.Fatalf("expected s3 hit, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(1, 7)
	if ok {
		t.Fatalf("expected no hit on gap row, got ok=%v action=%v id=%q", ok, action, id)
	}

	action, id, ok = sessions.MouseDown(1, 8)
	if !ok || action != SessionMouseSelect || id != "s1" {
		t.Fatalf("expected s1 hit, got ok=%v action=%v id=%q", ok, action, id)
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
