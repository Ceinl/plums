package components

import "testing"

func TestChatLogRendersMarkdownHeadingsAndBold(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 40, 10)
	cl.SetAiOutput("**Assessing next steps**\n\nBody with **important** text")

	lines := cl.lines()
	if len(lines) != 2 {
		t.Fatalf("expected 2 rendered lines, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "Assessing next steps" {
		t.Fatalf("expected bold markers removed, got %q", got)
	}
	if lines[0].spans[0].decor != decorBold {
		t.Fatalf("expected first line to be bold")
	}
	if got := spanText(lines[1].spans); got != "Body with important text" {
		t.Fatalf("expected inline bold markers removed, got %q", got)
	}
}

func TestChatLogRendersInlineCodeWithoutSemanticLineColoring(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("Verified with `go test ./...`.")

	lines := cl.lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 rendered line, got %d", len(lines))
	}
	if lines[0].contentFg != fgContent || lines[0].contentBg != "" {
		t.Fatalf("expected normal message colors")
	}
	if got := spanText(lines[0].spans); got != "Verified with go test ./...." {
		t.Fatalf("expected inline code markers removed, got %q", got)
	}
	if lines[0].spans[1].fg != fgInlineCode {
		t.Fatalf("expected inline code accent color")
	}
}

func TestChatLogDimsThinkingTraceAndRemovesThinkTags(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("<think>checking options</think> final answer")

	lines := cl.lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 rendered line, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "checking options final answer" {
		t.Fatalf("expected think tags removed, got %q", got)
	}
	if lines[0].spans[0].fg != fgThinking {
		t.Fatalf("expected thinking text to be dimmed")
	}
	if lines[0].spans[len(lines[0].spans)-1].fg != fgContent {
		t.Fatalf("expected final answer to use normal content color")
	}
}

func TestChatLogContinuesThinkingColorAcrossLines(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("<think>line one\nline two</think>\nanswer")

	lines := cl.lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 rendered lines, got %d", len(lines))
	}
	if lines[0].spans[0].fg != fgThinking || lines[1].spans[0].fg != fgThinking {
		t.Fatalf("expected thinking text to stay dimmed across lines")
	}
	if lines[2].spans[0].fg != fgContent {
		t.Fatalf("expected text after closing tag to return to normal color")
	}
}

func TestChatLogRendersListItemsWithHangingIndent(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 24, 10)
	cl.SetAiOutput("1. This is a longer list item that wraps\n- short")

	lines := cl.lines()
	if len(lines) < 3 {
		t.Fatalf("expected wrapped list lines, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "  1. This is a longer" {
		t.Fatalf("unexpected first list line: %q", got)
	}
	if got := spanText(lines[1].spans); got != "     list item that" {
		t.Fatalf("unexpected wrapped list line: %q", got)
	}
	if got := spanText(lines[len(lines)-1].spans); got != "  • short" {
		t.Fatalf("unexpected bullet line: %q", got)
	}
}

func TestChatLogGivesUserMessagesBackgroundAndAiMessagesPlainBackground(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetMessages([]ChatMessage{
		{Role: "user", Content: "do the thing"},
		{Role: "ai", Content: "done"},
	})

	lines := cl.lines()
	if len(lines) != 3 {
		t.Fatalf("expected user line, separator, and ai line, got %d", len(lines))
	}
	if lines[0].contentBg != bgUserMessage {
		t.Fatalf("expected user message background")
	}
	if lines[0].accentFg != fgUserAccent {
		t.Fatalf("expected user message accent")
	}
	if lines[2].contentBg != "" {
		t.Fatalf("expected ai message to use plain background")
	}
	if lines[2].accentFg != "" {
		t.Fatalf("expected ai message to have no accent")
	}
}

func TestChatLogSelectionSurvivesUnchangedSetters(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("hello world")

	cl.MouseDown(0, 0)
	cl.SetAiOutput("hello world")
	got := cl.MouseUp(5, 0)
	if got != "hello" {
		t.Fatalf("expected selected text after unchanged render update, got %q", got)
	}
}

func spanText(spans []textSpan) string {
	out := ""
	for _, span := range spans {
		out += span.text
	}
	return out
}
