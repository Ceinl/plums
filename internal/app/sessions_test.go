package app

import (
	"testing"

	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

func TestSessionMouseDownQueuesNewSessionAction(t *testing.T) {
	state := NewState(80, 24)
	sessions := state.Sessions()
	sessions.SetItems(nil)
	sessions.Layout(0, 0, 30, 10)
	sessions.Render(screen.NewScreen(80, 24))

	ctx := &fakeCtx{}
	if !state.SessionMouseDown(ctx, 1, 0) {
		t.Fatalf("expected new session button hit")
	}
	if !ctx.called("NewSession") {
		t.Fatalf("expected new session call, got %v", ctx.calls)
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

	ctx := &fakeCtx{}
	if !state.SessionMouseDown(ctx, 1, 3) {
		t.Fatalf("expected session mouse hit")
	}
	if !ctx.called("OpenSession") {
		t.Fatalf("expected open session call, got %v", ctx.calls)
	}
}

func TestVerticalAndHorizontalSessionsKeepSeparateHitMaps(t *testing.T) {
	state := NewState(100, 24)
	state.SessionItems = []SessionListItem{{ID: "s1", Title: "One"}, {ID: "s2", Title: "Two"}}

	vertical := newSessions(state, components.SessionsVertical)
	vertical.Layout(0, 0, 15, 24)
	vertical.Render(screen.NewScreen(100, 24))

	horizontal := newSessions(state, components.SessionsHorizontal)
	horizontal.Layout(20, 0, 80, 3)
	horizontal.Render(screen.NewScreen(120, 24))

	ctx := &fakeCtx{}
	if !state.SessionMouseDown(ctx, 1, 0) {
		t.Fatalf("expected vertical session to still hit after horizontal render")
	}
	if !ctx.called("NewSession") {
		t.Fatalf("expected new session call, got %v", ctx.calls)
	}
}

// newSessions populates and returns the State-owned Sessions widget for the
// given orientation, mirroring what sessionsComponent.Render does in prod. It
// lives here because only the hit-map tests need to drive the widget directly.
func newSessions(state *State, orientation components.SessionsOrientation) *components.Sessions {
	sessions := state.Sessions()
	if orientation == components.SessionsHorizontal {
		sessions = state.SessionsHorizontal()
	}
	sessions.SetOrientation(orientation)
	items := make([]components.SessionItem, 0, len(state.SessionItems))
	for _, item := range state.SessionItems {
		items = append(items, components.SessionItem{
			ID:        item.ID,
			Title:     item.Title,
			Directory: item.Directory,
			Updated:   item.Updated,
			Current:   item.Current || item.ID == state.SessionID,
		})
	}
	sessions.SetItems(items)
	return sessions
}
