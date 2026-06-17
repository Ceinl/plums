package claudemirror

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Ceinl/plums/internal/core/adapter"
)

func TestE2EManual(t *testing.T) {
	dir := os.Getenv("MIRROR_E2E_DIR")
	if dir == "" {
		t.Skip("set MIRROR_E2E_DIR to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	b := NewBackend()
	session, err := b.CreateSession(ctx, dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Logf("attached session=%s dir=%s title=%q", session.ID, session.Directory, session.Title)
	for i, prompt := range []string{"Reply with exactly: MIRROR_ONE", "Reply with exactly: MIRROR_TWO"} {
		got := ""
		for ev := range b.SendMessageEvents(ctx, session.ID, prompt, "", "", "") {
			if ev.Text != "" {
				t.Logf("turn %d TEXT: %q", i+1, ev.Text)
				got += ev.Text
			}
		}
		if got == "" {
			t.Fatalf("turn %d: no text mirrored back", i+1)
		}
	}

	// ResetSession starts a fresh conversation in the same window via /clear.
	reset, err := b.(interface {
		ResetSession(context.Context, string) (*adapter.Session, error)
	}).ResetSession(ctx, dir)
	if err != nil {
		t.Fatalf("ResetSession: %v", err)
	}
	t.Logf("reset session=%s", reset.ID)
	got := ""
	for ev := range b.SendMessageEvents(ctx, reset.ID, "Reply with exactly: MIRROR_THREE", "", "", "") {
		if ev.Text != "" {
			got += ev.Text
		}
	}
	if got == "" {
		t.Fatal("post-reset turn: no text mirrored back")
	}
}
