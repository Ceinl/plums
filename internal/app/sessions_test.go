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

func TestChatLayoutVerticalSessionsNewAction(t *testing.T) {
	state := NewState(120, 24)
	state.Layout = LayoutChat
	state.SessionItems = []SessionListItem{{ID: "s1", Title: "One"}}

	root := CreateSessionsLayout(state)
	root.Layout(0, 0, 100, 24)
	root.Render(screen.NewScreen(100, 24))

	ctx := &fakeCtx{}
	if !state.SessionMouseDown(ctx, 1, 0) {
		t.Fatalf("expected vertical sessions new button hit")
	}
	if !ctx.called("NewSession") {
		t.Fatalf("expected new session call, got %v", ctx.calls)
	}
}

func TestNarrowChatLayoutHorizontalSessionsActions(t *testing.T) {
	state := NewState(80, 24)
	state.Layout = LayoutChat
	state.SessionItems = []SessionListItem{{ID: "s1", Title: "One"}, {ID: "s2", Title: "Two"}}

	root := CreateNarrowSessionsLayout(state)
	root.Layout(0, 0, 80, 24)
	root.Render(screen.NewScreen(80, 24))

	ctx := &fakeCtx{}
	if !state.SessionMouseDown(ctx, 1, 1) {
		t.Fatalf("expected horizontal sessions tab hit")
	}
	if !ctx.called("OpenSession") {
		t.Fatalf("expected open session call, got %v", ctx.calls)
	}

	root.Render(screen.NewScreen(80, 24))
	ctx = &fakeCtx{}
	if !state.SessionMouseDown(ctx, 78, 1) {
		t.Fatalf("expected horizontal sessions new button hit")
	}
	if !ctx.called("NewSession") {
		t.Fatalf("expected new session call, got %v", ctx.calls)
	}
}

func TestConfiguredChatLayoutsSessionsActions(t *testing.T) {
	cfg := testRenderConfig(t)

	wide := NewState(100, 24)
	wide.Layout = LayoutChat
	wide.SessionItems = []SessionListItem{{ID: "s1", Title: "One"}}
	root, err := buildLayout(wide, cfg, "chat")
	if err != nil {
		t.Fatalf("build wide chat layout: %v", err)
	}
	root.Layout(0, 0, 100, 24)
	root.Render(screen.NewScreen(100, 24))
	ctx := &fakeCtx{}
	if !wide.SessionMouseDown(ctx, 1, 0) {
		t.Fatalf("expected configured vertical sessions new button hit")
	}
	if !ctx.called("NewSession") {
		t.Fatalf("expected new session call, got %v", ctx.calls)
	}

	narrow := NewState(80, 24)
	narrow.Layout = LayoutChat
	narrow.SessionItems = []SessionListItem{{ID: "s1", Title: "One"}, {ID: "s2", Title: "Two"}}
	root, err = buildLayout(narrow, cfg, "chat")
	if err != nil {
		t.Fatalf("build narrow chat layout: %v", err)
	}
	root.Layout(0, 0, 80, 24)
	root.Render(screen.NewScreen(80, 24))
	ctx = &fakeCtx{}
	if !narrow.SessionMouseDown(ctx, 1, 1) {
		t.Fatalf("expected configured horizontal sessions tab hit")
	}
	if !ctx.called("OpenSession") {
		t.Fatalf("expected open session call, got %v", ctx.calls)
	}
	root.Render(screen.NewScreen(80, 24))
	ctx = &fakeCtx{}
	if !narrow.SessionMouseDown(ctx, 78, 1) {
		t.Fatalf("expected configured horizontal sessions new button hit")
	}
	if !ctx.called("NewSession") {
		t.Fatalf("expected new session call, got %v", ctx.calls)
	}
}

func TestActiveConfiguredChatLayoutsSessionsActions(t *testing.T) {
	cfg := testRenderConfig(t)

	state := NewState(80, 24)
	state.Layout = LayoutChat
	state.SessionItems = []SessionListItem{{ID: "s1", Title: "One"}, {ID: "s2", Title: "Two"}}
	root, err := buildLayout(state, cfg, "chat")
	if err != nil {
		t.Fatalf("build active narrow chat layout: %v", err)
	}
	root.Layout(0, 0, 80, 24)
	root.Render(screen.NewScreen(80, 24))
	ctx := &fakeCtx{}
	if !state.SessionMouseDown(ctx, 1, 1) {
		t.Fatalf("expected active configured horizontal sessions tab hit")
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
