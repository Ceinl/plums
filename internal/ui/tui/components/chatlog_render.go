package components

import (
	"fmt"
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/screen"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// ── Render ────────────────────────────────────────────────────────────────────

func (cl *ChatLog) Render(s *screen.Screen) {
	bg := cl.style.GetBackground()
	if cl.parent != nil {
		bg = cl.parent.GetStyle().GetBackground()
	}

	if len(cl.messages) == 0 && cl.aioutput == "" && !cl.isStreaming {
		cl.renderEmptyState(s, bg)
		return
	}

	// Build the full list of logical lines (may be taller than cl.h).
	lines := cl.lines()
	maxScroll := len(lines) - cl.h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if cl.onMaxScroll != nil {
		cl.onMaxScroll(maxScroll)
	}
	scrollOffset := cl.scrollOffset
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	// scrollOffset is the distance from the bottom. Zero preserves auto-scroll.
	start := 0
	if len(lines) > cl.h {
		maxStart := len(lines) - cl.h
		start = maxStart - scrollOffset
		if start < 0 {
			start = 0
		}
	}

	for row := 0; row < cl.h; row++ {
		y := cl.y + row
		idx := start + row
		if idx < len(lines) {
			cl.renderLine(s, y, lines[idx], bg)
		} else {
			cl.clearRow(s, y, bg)
		}
	}
}

// renderEmptyState fills the chat area and draws a centred placeholder so an
// empty conversation is not just a dark void.
func (cl *ChatLog) renderEmptyState(s *screen.Screen, bg string) {
	for row := 0; row < cl.h; row++ {
		cl.clearRow(s, cl.y+row, bg)
	}
	if cl.onMaxScroll != nil {
		cl.onMaxScroll(0)
	}
	if cl.w < 10 || cl.h < 3 {
		return
	}
	mid := cl.y + cl.h/2 - 1
	drawCenteredText(s, cl.x, mid, cl.w, "✳ plums", theme.Accent.Fg(), bg)
	drawCenteredText(s, cl.x, mid+2, cl.w, "type a message below to start", theme.TextFaint.Fg(), bg)
}

// buildMessageLines converts committed messages into a flat slice of renderLine
// values that represent one terminal row each. The result is cached and reused
// across frames (see messageLines); only buildStreamingLines runs per token.
func (cl *ChatLog) buildMessageLines() []renderLine {
	var lines []renderLine

	for i, msg := range cl.messages {
		if i > 0 {
			lines = append(lines, renderLine{kind: lineKindBlank})
		}
		bodyFg, bodyBg := cl.roleStyle(msg.Role)
		if msg.Role == "system" {
			lines = append(lines, renderLine{kind: lineKindHeader, role: "!", roleFg: fgSystemRole})
		}
		accentFg := ""
		if msg.Role == "user" {
			accentFg = fgUserAccent
		}
		lines = append(lines, withAccent(cl.buildContentLines(msg.Content, bodyFg, bodyBg), accentFg)...)
	}

	return lines
}

// buildStreamingLines renders the in-progress AI response that is appended below
// the committed messages while a turn is streaming.
func (cl *ChatLog) buildStreamingLines() []renderLine {
	if cl.aioutput == "" && !cl.isStreaming {
		return nil
	}
	var lines []renderLine
	if len(cl.messages) > 0 {
		lines = append(lines, renderLine{kind: lineKindBlank})
	}
	if cl.aioutput != "" {
		content := cl.aioutput
		if cl.isStreaming {
			content += "▌"
		}
		lines = append(lines, cl.buildContentLines(content, fgContent, "")...)
	}
	return lines
}

func (cl *ChatLog) buildContentLines(content, fg, bg string) []renderLine {
	blocks := parseChatBlocks(content)
	var lines []renderLine

	for i := 0; i < len(blocks); i++ {
		block := blocks[i]
		switch block.kind {
		case blockKindCode:
			bgCodeBlock := theme.BgSurface.Bg()
			langLabel := block.lang
			if langLabel == "" {
				langLabel = "code"
			}
			headerText := "╭─ " + langLabel + " ─"
			headerSpans := []textSpan{{text: headerText, fg: fgCodeFence}}
			lines = append(lines, renderLine{kind: lineKindContent, spans: headerSpans, contentFg: fgCodeFence, contentBg: bgCodeBlock})

			for _, spans := range wrapSpanLines(highlightCode(block.lang, block.text, fg), cl.contentWidth()) {
				lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bgCodeBlock})
			}

			footerSpans := []textSpan{{text: "╰" + strings.Repeat("─", 20), fg: fgCodeFence}}
			lines = append(lines, renderLine{kind: lineKindContent, spans: footerSpans, contentFg: fgCodeFence, contentBg: bgCodeBlock})

		case blockKindToolCall, blockKindToolOutput:
			// Consume the whole run of consecutive tool blocks so collapse
			// mode can summarise multiple calls as one line.
			run := 1
			for i+run < len(blocks) {
				k := blocks[i+run].kind
				if k != blockKindToolCall && k != blockKindToolOutput {
					break
				}
				run++
			}
			lines = append(lines, cl.buildToolLines(blocks[i:i+run], bg)...)
			i += run - 1

		case blockKindText:
			lines = append(lines, cl.buildMarkdownLines(block.text, fg, bg)...)
		}
	}

	return lines
}

// buildToolLines renders a run of consecutive tool-call/output blocks according
// to the active visibility mode. In collapse mode a run with more than one call
// is reduced to a single summary line; hidden mode drops the run entirely.
func (cl *ChatLog) buildToolLines(blocks []chatBlock, bg string) []renderLine {
	switch cl.toolCallMode {
	case ToolCallVisibilityHidden:
		return nil

	case ToolCallVisibilityCollapse:
		var names []string
		for _, b := range blocks {
			if b.kind == blockKindToolCall {
				names = append(names, toolDisplayName(b.toolName))
			}
		}
		if len(names) <= 1 {
			// A lone call: show just the call line, no verbose output.
			return cl.toolCallLines(blocks, bg, false)
		}
		summary := fmt.Sprintf("%d tool calls", len(names))
		spans := []textSpan{
			{text: "● ", fg: fgToolIndicator},
			{text: summary, fg: fgToolCall, decor: decorBold},
		}
		joined := strings.Join(names, ", ")
		avail := cl.contentWidth() - 3 - len([]rune(summary))
		spans = append(spans, textSpan{text: " " + truncateToolSummary(joined, avail), fg: fgToolArg})
		return []renderLine{{kind: lineKindContent, spans: spans, contentFg: fgToolCall, contentBg: bg}}

	default:
		return cl.toolCallLines(blocks, bg, true)
	}
}

// toolCallLines renders each call (and, when withOutput is set, its response)
// as its own line.
func (cl *ChatLog) toolCallLines(blocks []chatBlock, bg string, withOutput bool) []renderLine {
	var lines []renderLine
	for _, block := range blocks {
		switch block.kind {
		case blockKindToolCall:
			name := toolDisplayName(block.toolName)
			spans := []textSpan{
				{text: "● ", fg: fgToolIndicator},
				{text: name, fg: fgToolCall, decor: decorBold},
			}
			if arg := toolCallArg(block.text); arg != "" {
				avail := cl.contentWidth() - 3 - len([]rune(name))
				spans = append(spans, textSpan{text: " " + truncateToolSummary(arg, avail), fg: fgToolArg})
			}
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fgToolCall, contentBg: bg})

		case blockKindToolOutput:
			if !withOutput {
				continue
			}
			outputCompact := compactToOneLine(block.text)
			if outputCompact == "" {
				continue
			}
			displayText := truncateToolSummary(outputCompact, cl.contentWidth()-4)
			spans := []textSpan{
				{text: "  ⎿ ", fg: fgToolMarker},
				{text: displayText, fg: fgToolOutput},
			}
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fgToolOutput, contentBg: bg})
		}
	}
	return lines
}

func toolDisplayName(name string) string {
	if name == "" {
		return "tool"
	}
	return name
}

func (cl *ChatLog) roleStyle(role string) (bodyFg, bodyBg string) {
	switch role {
	case "user":
		// Inherit the surrounding background; the amber accent bar (fgUserAccent)
		// is what distinguishes a user prompt, so it stays legible on any layout
		// palette instead of imposing a fixed panel colour.
		return fgContent, ""
	case "ai":
		return fgContent, ""
	case "system":
		return fgSystemBody, ""
	default:
		return fgContent, ""
	}
}

func withAccent(lines []renderLine, accentFg string) []renderLine {
	if accentFg == "" {
		return lines
	}
	for i := range lines {
		if lines[i].kind == lineKindContent {
			lines[i].accentFg = accentFg
		}
	}
	return lines
}

func (cl *ChatLog) contentWidth() int {
	w := cl.w - 2 // 2-space indent for body lines
	if w < 1 {
		w = 1
	}
	return w
}

func (cl *ChatLog) buildListLines(marker, body, fg, bg string) []renderLine {
	markerRunes := []rune(strings.Repeat(" ", listIndent) + marker + " ")
	indent := len(markerRunes)
	available := cl.contentWidth() - indent
	if available < 1 {
		available = 1
	}

	wrapped := wrapInlineMarkdown(body, available, fg, "")
	if len(wrapped) == 0 {
		wrapped = [][]textSpan{{}}
	}
	lines := make([]renderLine, 0, len(wrapped))
	for i, spans := range wrapped {
		prefix := markerRunes
		if i > 0 {
			prefix = []rune(strings.Repeat(" ", indent))
		}
		combined := []textSpan{{text: string(prefix), fg: fgListMarker}}
		combined = append(combined, spans...)
		lines = append(lines, renderLine{kind: lineKindContent, spans: combined, contentFg: fg, contentBg: bg})
	}
	return lines
}

// ── Per-row rendering ─────────────────────────────────────────────────────────

func (cl *ChatLog) renderLine(s *screen.Screen, y int, line renderLine, bg string) {
	switch line.kind {
	case lineKindBlank:
		cl.clearRow(s, y, bg)
	case lineKindHeader:
		cl.renderHeader(s, y, line.role, line.roleFg, bg)
	case lineKindContent:
		fg := fgContent
		if line.contentFg != "" {
			fg = line.contentFg
		}
		lineBg := bg
		if line.contentBg != "" {
			lineBg = line.contentBg
		}
		cl.renderContent(s, y, line.text, line.spans, fg, lineBg, line.accentFg)
	}
}

// renderHeader draws system markers across the full row.
func (cl *ChatLog) selectionFgBg(y, x int, fg, bg string) (string, string) {
	rx1, rx2, ok := cl.rowSelectionRange(y)
	if !ok {
		return fg, bg
	}
	if x >= rx1 && x < rx2 {
		return selFg, selBg
	}
	return fg, bg
}

func (cl *ChatLog) renderHeader(s *screen.Screen, y int, role string, roleFg string, bg string) {
	x := cl.x

	// Role label.
	for _, r := range []rune(role) {
		if x >= cl.x+cl.w {
			break
		}
		cellFg, cellBg := cl.selectionFgBg(y, x, roleFg, bg)
		s.Set(x, y, r, cellFg, cellBg, "")
		x++
	}

	// Space between marker and rule.
	if x < cl.x+cl.w {
		cellFg, cellBg := cl.selectionFgBg(y, x, fgDimRule, bg)
		s.Set(x, y, ' ', cellFg, cellBg, "")
		x++
	}

	// Horizontal rule filling the rest.
	for x < cl.x+cl.w {
		cellFg, cellBg := cl.selectionFgBg(y, x, fgDimRule, bg)
		s.Set(x, y, '─', cellFg, cellBg, "")
		x++
	}
}

// renderContent draws "  <text><padding>" using the given foreground colour.
func (cl *ChatLog) renderContent(s *screen.Screen, y int, text string, spans []textSpan, fg string, bg string, accentFg string) {
	x := cl.x
	if accentFg != "" && x < cl.x+cl.w {
		cellFg, cellBg := cl.selectionFgBg(y, x, accentFg, bg)
		s.Set(x, y, '▏', cellFg, cellBg, "")
		x++
	}

	// 2-space indent.
	for i := 0; i < 2 && x < cl.x+cl.w; i++ {
		cellFg, cellBg := cl.selectionFgBg(y, x, fg, bg)
		s.Set(x, y, ' ', cellFg, cellBg, "")
		x++
	}

	if len(spans) == 0 {
		spans = []textSpan{{text: text, fg: fg}}
	}

	for spanIdx, span := range spans {
		cellFg := span.fg
		if cellFg == "" {
			cellFg = fg
		}

		runes := []rune(span.text)
		for i, r := range runes {
			if x >= cl.x+cl.w {
				break
			}
			if r == '\t' {
				spaces := 4 - ((x - cl.x - 2) % 4)
				for ; spaces > 0 && x < cl.x+cl.w; spaces-- {
					cf, cb := cl.selectionFgBg(y, x, cellFg, bg)
					s.Set(x, y, ' ', cf, cb, span.decor)
					x++
				}
				continue
			}
			if r == '▌' && spanIdx == len(spans)-1 && i == len(runes)-1 {
				cellFg = fgCursor
			}
			cf, cb := cl.selectionFgBg(y, x, cellFg, bg)
			s.Set(x, y, sanitizeRenderableRune(r), cf, cb, span.decor)
			x++
		}
		if x >= cl.x+cl.w {
			break
		}
	}

	// Fill remainder.
	for x < cl.x+cl.w {
		cellFg, cellBg := cl.selectionFgBg(y, x, fg, bg)
		s.Set(x, y, ' ', cellFg, cellBg, "")
		x++
	}
}

func sanitizeRenderableRune(r rune) rune {
	if r == 0x7f || (r >= 0x00 && r <= 0x1f) {
		return '�'
	}
	return r
}

func compactBlankLines(lines []renderLine) []renderLine {
	out := lines[:0]
	previousBlank := true
	for _, line := range lines {
		if line.kind == lineKindBlank {
			if previousBlank {
				continue
			}
			previousBlank = true
			out = append(out, line)
			continue
		}
		previousBlank = false
		out = append(out, line)
	}
	if len(out) > 0 && out[len(out)-1].kind == lineKindBlank {
		out = out[:len(out)-1]
	}
	return out
}

func (cl *ChatLog) clearRow(s *screen.Screen, y int, bg string) {
	for x := cl.x; x < cl.x+cl.w; x++ {
		cellFg, cellBg := cl.selectionFgBg(y, x, fgContent, bg)
		s.Set(x, y, ' ', cellFg, cellBg, "")
	}
}
