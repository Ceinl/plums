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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b := NewBackend()
	session, err := b.CreateSession(ctx, dir)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Logf("attached session=%s dir=%s title=%q", session.ID, session.Directory, session.Title)
	events := b.SendMessageEvents(ctx, session.ID, "Reply with exactly: MIRROR_OK", "", "", "")
	got := ""
	for ev := range events {
		if ev.Text != "" {
			t.Logf("TEXT: %q", ev.Text)
			got += ev.Text
		}
		if ev.Tool != nil {
			t.Logf("TOOL: %s in=%q out=%q", ev.Tool.Name, ev.Tool.Input, ev.Tool.Output)
		}
	}
	if got == "" {
		t.Fatal("no text mirrored back")
	}
}
