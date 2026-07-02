package components

import (
	"strings"
	"testing"
)

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

func TestChatLogStreamingShowsStatusBeforeFirstToken(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetStreaming(true)

	lines := cl.lines()
	if len(lines) != 1 {
		t.Fatalf("expected streaming status line, got %d lines", len(lines))
	}
	if got := spanText(lines[0].spans); got != "● responding waiting for output ▌" {
		t.Fatalf("status text = %q", got)
	}
}

func TestChatLogStreamingContentIsAccented(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetStreaming(true)
	cl.SetAiOutput("partial answer")

	lines := cl.lines()
	if len(lines) != 2 {
		t.Fatalf("expected status and content lines, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != "● responding" {
		t.Fatalf("status text = %q", got)
	}
	if got := spanText(lines[1].spans); got != "partial answer ▌" {
		t.Fatalf("content text = %q", got)
	}
	if lines[1].accentFg != fgCursor {
		t.Fatalf("streaming content accent = %q, want %q", lines[1].accentFg, fgCursor)
	}
}

func TestChatLogDimsThinkingTraceAndRemovesThinkTags(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput("<thinking>checking options</thinking> final answer")

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
	cl.SetAiOutput("<thinking>line one\nline two</thinking>\nanswer")

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
	cl.SetAiOutput("<thinking>**bold** `code`</thinking>")

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
	cl.SetAiOutput("<thinking>line one\nline two</thinking> answer")

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
	cl.SetAiOutput("<thinking>hidden</thinking> answer")

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
	cl.SetAiOutput("before <thinking>one</thinking> after <thinking>two</thinking> done")

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
	cl.SetAiOutput("<thinking>one </thinking><thinking>two </thinking><thinking>three</thinking> answer")

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

func TestChatLogDistinguishesUserMessagesByAccentNotBackground(t *testing.T) {
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
	// User messages inherit the surrounding background (no imposed panel) so the
	// chat blends into whichever layout palette is active; the accent bar is the
	// sole distinguisher.
	if lines[0].contentBg != "" {
		t.Fatalf("expected user message to inherit background, got %q", lines[0].contentBg)
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

func TestChatLogRendersPlainToolCallTranscriptAsSingleLine(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetAiOutput(`Called the Read tool with the following input: {"filePath":"/tmp/example.go"}`)

	lines := cl.lines()
	if len(lines) != 1 {
		t.Fatalf("expected 1 rendered line, got %d", len(lines))
	}
	if got := spanText(lines[0].spans); got != `● Read /tmp/example.go` {
		t.Fatalf("expected tool call summary, got %q", got)
	}
	if lines[0].spans[0].fg != fgToolIndicator || lines[0].spans[1].fg != fgToolCall {
		t.Fatalf("expected tool call colors")
	}
}

func TestChatLogCollapsesConsecutiveToolCalls(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetToolCallVisibility(ToolCallVisibilityCollapse)
	cl.SetAiOutput("text\n" +
		`<tool_call name="read">{"file_path":"/a.go"}</tool_call>` + "\n" +
		"<tool_output>contents of a</tool_output>\n" +
		`<tool_call name="grep">{"pattern":"foo"}</tool_call>` + "\n" +
		"<tool_output>1 match</tool_output>\n")

	lines := cl.lines()
	var collapsed string
	for _, l := range lines {
		if got := spanText(l.spans); strings.Contains(got, "tool calls") {
			collapsed = got
		}
	}
	if collapsed != "● 2 tool calls read, grep" {
		t.Fatalf("expected collapsed summary, got %q", collapsed)
	}
}

func TestChatLogHidesToolCalls(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 80, 10)
	cl.SetToolCallVisibility(ToolCallVisibilityHidden)
	cl.SetAiOutput(`<tool_call name="read">{"file_path":"/a.go"}</tool_call>` + "\n" +
		"<tool_output>contents</tool_output>\n")

	for _, l := range cl.lines() {
		if got := spanText(l.spans); strings.Contains(got, "read") {
			t.Fatalf("expected tool call hidden, got line %q", got)
		}
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

func TestChatLogRendersMarkdownTable(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 60, 10)
	cl.SetAiOutput("| Name | Age | City |\n|------|-----|------|\n| Alice | 30 | NYC |\n| Bob | 25 | LA |")

	lines := cl.lines()
	if len(lines) != 4 {
		t.Fatalf("expected 4 table rows, got %d", len(lines))
	}

	// Header row should have bold/heading styling via inline markdown? No, pipes use fgCodeFence
	// Check that the structure is reasonable — at least pipes are present
	headerText := spanText(lines[0].spans)
	if !strings.Contains(headerText, "|") {
		t.Fatalf("expected header row to contain pipes, got %q", headerText)
	}
	if !strings.Contains(headerText, "Name") {
		t.Fatalf("expected header row to contain 'Name', got %q", headerText)
	}

	sepText := spanText(lines[1].spans)
	if !strings.Contains(sepText, "─") {
		t.Fatalf("expected separator row to contain dashes, got %q", sepText)
	}

	// Verify columns are aligned by checking positions of pipes or cell content
	row3 := spanText(lines[2].spans)
	if !strings.Contains(row3, "Alice") || !strings.Contains(row3, "30") {
		t.Fatalf("expected data row with Alice and 30, got %q", row3)
	}

	row4 := spanText(lines[3].spans)
	if !strings.Contains(row4, "Bob") || !strings.Contains(row4, "25") {
		t.Fatalf("expected data row with Bob and 25, got %q", row4)
	}
}

func TestChatLogTableTruncatesWhenTooWide(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 20, 10)
	cl.SetAiOutput("| VeryLongColumnA | VeryLongColumnB |\n| data | more |")

	lines := cl.lines()
	if len(lines) < 2 {
		t.Fatalf("expected at least header and data rows, got %d", len(lines))
	}

	totalWidth := 0
	for _, span := range lines[0].spans {
		totalWidth += len([]rune(span.text))
	}
	if totalWidth > 20 {
		t.Fatalf("expected table to fit within 20 cols, got width %d: %q", totalWidth, spanText(lines[0].spans))
	}
}

func TestChatLogTableFallsBackToTextWhenExtremelyNarrow(t *testing.T) {
	cl := NewChatLog()
	cl.Layout(0, 0, 4, 10)
	cl.SetAiOutput("| a | b |\n| 1 | 2 |")

	lines := cl.lines()
	if len(lines) == 0 {
		t.Fatalf("expected some output")
	}
	// When too narrow it should fall back to plain text rendering
	got := spanText(lines[0].spans)
	if !strings.Contains(got, "a") {
		t.Fatalf("expected fallback text to contain 'a', got %q", got)
	}
}

func spanText(spans []textSpan) string {
	out := ""
	for _, span := range spans {
		out += span.text
	}
	return out
}
