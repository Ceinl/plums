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
	if len(lines) != 2 {
		t.Fatalf("expected 2 rendered lines, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "checking options" {
		t.Fatalf("expected think tags removed, got %q", got)
	}
	if lines[0].spans[0].fg != fgThinking {
		t.Fatalf("expected thinking text to be dimmed")
	}
	if got := spanText(lines[1].spans); got != "final answer" {
		t.Fatalf("expected final answer on next line, got %q", got)
	}
	if lines[1].spans[0].fg != fgContent {
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

func TestChatLogThinkingIgnoresInlineDecorators(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("<think>**bold** `code`</think>")

	lines := cl.lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 rendered line, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "bold code" {
		t.Fatalf("expected markdown markers removed, got %q", got)
	}
	for _, span := range lines[0].spans {
		if span.fg != fgThinking || span.decor != "" {
			t.Fatalf("expected plain thinking style, got fg=%q decor=%q", span.fg, span.decor)
		}
	}
}

func TestChatLogCanShowOnlyThinkingTitle(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetThinkingVisibility(ThinkingVisibilityTitle)
	cl.SetAiOutput("<think>line one\nline two</think> answer")

	lines := cl.lines()
	if len(lines) != 2 {
		t.Fatalf("expected title and answer lines, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "thinking..." {
		t.Fatalf("expected thinking title, got %q", got)
	}
	if got := spanText(lines[1].spans); got != "answer" {
		t.Fatalf("expected answer, got %q", got)
	}
}

func TestChatLogCanHideThinkingEntirely(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetThinkingVisibility(ThinkingVisibilityHidden)
	cl.SetAiOutput("<think>hidden</think> answer")

	lines := cl.lines()
	if len(lines) != 1 {
		t.Fatalf("expected answer line only, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "answer" {
		t.Fatalf("expected only non-thinking text, got %q", got)
	}
}

func TestChatLogStartsNewLineAfterEachThinkingPart(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("before <think>one</think> after <think>two</think> done")

	lines := cl.lines()
	if len(lines) != 3 {
		t.Fatalf("expected each thinking boundary on its own line, got %d", len(lines))
	}
	want := []string{"before one", "after two", "done"}
	for i, line := range lines {
		if got := spanText(line.spans); got != want[i] {
			t.Fatalf("line %d: expected %q, got %q", i, want[i], got)
		}
	}
}

func TestChatLogMergesAdjacentThinkingChunks(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("<think>one </think><think>two </think><think>three</think> answer")

	lines := cl.lines()
	if len(lines) != 2 {
		t.Fatalf("expected merged thinking line and answer line, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "one two three" {
		t.Fatalf("expected adjacent thinking chunks merged, got %q", got)
	}
	if got := spanText(lines[1].spans); got != "answer" {
		t.Fatalf("expected answer line, got %q", got)
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
