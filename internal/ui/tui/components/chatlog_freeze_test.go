package components

import (
	"strings"
	"testing"
	"time"
)

func bigHistory(n int) []ChatMessage {
	msgs := make([]ChatMessage, 0, n)
	for i := 0; i < n; i++ {
		msgs = append(msgs, ChatMessage{
			Role:    "ai",
			Content: "Some explanation paragraph.\n\n```go\nfunc f() { fmt.Println(\"x\") }\n```\n\nmore text",
		})
	}
	return msgs
}

// Appending streamed output must not invalidate the cached, already-highlighted
// committed messages — otherwise every token re-highlights the whole history.
func TestStreamingDoesNotRebuildHistory(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 100, 40)
	cl.SetMessages(bigHistory(40))

	_ = cl.lines() // warm the committed-message cache
	if !cl.msgLinesCached {
		t.Fatal("expected committed-message cache to be warm")
	}

	cl.SetAiOutput("streaming tokens so far")
	cl.SetStreaming(true)
	if !cl.msgLinesCached {
		t.Fatal("streaming output invalidated the committed-message cache (history re-highlighted per token)")
	}
}

// A single rendered frame during streaming must not scale with history size,
// since the committed messages are highlighted once and reused.
func TestStreamingFrameCostIndependentOfHistory(t *testing.T) {
	measure := func(history int) time.Duration {
		cl := NewChatLog()
		cl.Layout(0, 0, 100, 40)
		cl.SetMessages(bigHistory(history))
		cl.SetStreaming(true)
		cl.SetAiOutput("Here is the code:\n\n```go\n" + strings.Repeat("foo bar baz ", 50) + "\n```")
		_ = cl.lines() // warm cache

		const frames = 200
		start := time.Now()
		for i := 0; i < frames; i++ {
			cl.SetAiOutput("Here is the code:\n\n```go\n" + strings.Repeat("foo bar baz ", 50+i%3) + "\n```")
			_ = cl.lines()
		}
		return time.Since(start) / frames
	}

	small := measure(5)
	large := measure(200)
	t.Logf("per-frame: small-history=%s large-history=%s", small, large)

	// A 40x larger history should not blow up per-frame cost. Allow generous
	// slack for noise but catch full-history re-highlighting (which would make
	// large ~40x small).
	if large > small*4+time.Millisecond {
		t.Fatalf("frame cost scales with history: small=%s large=%s", small, large)
	}
}
