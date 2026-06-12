package claudemirror

import (
	"context"
	"os"
	"testing"
	"time"
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
}
