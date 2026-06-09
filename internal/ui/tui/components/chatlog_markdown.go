package components

import (
	"strings"
)

func (cl *ChatLog) buildMarkdownLines(content, fg, bg string) []renderLine {
	var lines []renderLine
	inThinking := false
	paras := strings.Split(content, "\n")
	for i := 0; i < len(paras); i++ {
		trimmed := strings.TrimSpace(paras[i])
		if trimmed == "" {
			continue
		}

		thinkingLines, nextThinking, hasThinkingMarkup := parseThinkingSpanLines(trimmed, fg, inThinking, cl.thinkingMode)
		inThinking = nextThinking
		if hasThinkingMarkup {
			for _, spanLine := range thinkingLines {
				for _, spans := range wrapSpansHard(spanLine.spans, cl.contentWidth()) {
					if len(spans) == 0 || spanTextEmpty(spans) {
						continue
					}
					lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg, accentFg: fgThinking})
				}
			}
			continue
		}

		if !inThinking && isTableRow(trimmed) {
			var rawRows []string
			var parsedRows [][]string
			j := i
			for j < len(paras) {
				rowTrimmed := strings.TrimSpace(paras[j])
				if rowTrimmed == "" {
					break
				}
				if isTableRow(rowTrimmed) {
					rawRows = append(rawRows, rowTrimmed)
					parsedRows = append(parsedRows, parseTableCells(rowTrimmed))
					j++
				} else {
					break
				}
			}
			if len(parsedRows) >= 2 {
				lines = append(lines, cl.buildTableLines(rawRows, parsedRows, fg, bg)...)
				i = j - 1
				continue
			}
		}

		if heading, ok := parseHeading(trimmed); ok {
			for _, spans := range wrapInlineMarkdown(heading, cl.contentWidth(), fgHeading, decorBold) {
				lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fgHeading, contentBg: bg})
			}
			continue
		}

		if marker, body, ok := parseListItem(trimmed); ok {
			lines = append(lines, cl.buildListLines(marker, body, fg, bg)...)
			continue
		}

		for _, spans := range wrapInlineMarkdown(trimmed, cl.contentWidth(), fg, "") {
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg})
		}
	}
	return compactBlankLines(lines)
}

func parseThinkingSpans(text, fg string, inThinking bool, mode ThinkingVisibility) ([]textSpan, bool, bool) {
	lines, nextThinking, hasMarkup := parseThinkingSpanLines(text, fg, inThinking, mode)
	if len(lines) == 0 {
		return nil, nextThinking, hasMarkup
	}
	return lines[0].spans, nextThinking, hasMarkup
}

func parseThinkingSpanLines(text, fg string, inThinking bool, mode ThinkingVisibility) ([]spanLine, bool, bool) {
	var spans []textSpan
	var lines []spanLine
	hasMarkup := inThinking || strings.Contains(text, "<thinking>") || strings.Contains(text, "</thinking>")
	startedInThinking := inThinking
	remaining := text
	titleShown := false
	flushLine := func() {
		lines = append(lines, spanLine{spans: spans})
		spans = nil
	}
	appendThinking := func(value string) {
		switch mode {
		case ThinkingVisibilityHidden:
			return
		case ThinkingVisibilityTitle:
			if startedInThinking || titleShown {
				return
			}
			spans = append(spans, textSpan{text: "thinking...", fg: fgThinking})
			titleShown = true
		default:
			spans = append(spans, parseInlineMarkdownPlainStyle(value, fgThinking)...)
		}
	}
	appendNormal := func(value string) {
		if len(spans) == 0 {
			value = strings.TrimLeft(value, " \t")
		}
		if value != "" {
			spans = append(spans, parseInlineMarkdown(value, fg, "")...)
		}
	}

	for remaining != "" {
		openIdx := strings.Index(remaining, "<thinking>")
		closeIdx := strings.Index(remaining, "</thinking>")

		if inThinking {
			if closeIdx == -1 {
				appendThinking(remaining)
				if len(spans) > 0 || len(lines) == 0 {
					flushLine()
				}
				return lines, true, hasMarkup
			}
			if closeIdx > 0 {
				appendThinking(remaining[:closeIdx])
			}
			remaining = remaining[closeIdx+len("</thinking>"):]
			inThinking = false
			if !strings.HasPrefix(remaining, "<thinking>") && (len(spans) > 0 || mode != ThinkingVisibilityHidden) {
				flushLine()
			}
			continue
		}

		if openIdx == -1 {
			appendNormal(remaining)
			if len(spans) > 0 || len(lines) == 0 {
				flushLine()
			}
			return lines, false, hasMarkup
		}
		if closeIdx != -1 && closeIdx < openIdx {
			if closeIdx > 0 {
				appendNormal(remaining[:closeIdx])
			}
			remaining = remaining[closeIdx+len("</thinking>"):]
			continue
		}
		if openIdx > 0 {
			appendNormal(remaining[:openIdx])
		}
		remaining = remaining[openIdx+len("<thinking>"):]
		inThinking = true
	}

	if len(spans) == 0 {
		spans = append(spans, textSpan{text: "", fg: fg})
	}
	flushLine()
	return lines, inThinking, hasMarkup
}

func spanTextEmpty(spans []textSpan) bool {
	for _, span := range spans {
		if span.text != "" {
			return false
		}
	}
	return true
}

func parseHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	level := 0
	for _, r := range line {
		if r != '#' {
			break
		}
		level++
	}
	if level == 0 || level > 6 || len(line) <= level || line[level] != ' ' {
		return "", false
	}
	return strings.TrimSpace(line[level:]), true
}

func parseListItem(line string) (marker, body string, ok bool) {
	if len(line) >= 2 && (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")) {
		return "•", strings.TrimSpace(line[2:]), true
	}

	dot := strings.IndexRune(line, '.')
	if dot <= 0 || dot > 3 || dot+1 >= len(line) || line[dot+1] != ' ' {
		return "", "", false
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return "", "", false
		}
	}
	return line[:dot+1], strings.TrimSpace(line[dot+2:]), true
}

func parseInlineMarkdown(text, fg, decor string) []textSpan {
	var spans []textSpan
	bold := false
	code := false
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '`' {
			if start < i {
				spans = append(spans, inlineSpan(text[start:i], fg, decor, bold, code))
			}
			code = !code
			start = i + 1
			continue
		}
		if i+1 >= len(text) || text[i] != '*' || text[i+1] != '*' {
			continue
		}
		if start < i {
			spans = append(spans, inlineSpan(text[start:i], fg, decor, bold, code))
		}
		bold = !bold
		i++
		start = i + 1
	}
	if start < len(text) {
		spans = append(spans, inlineSpan(text[start:], fg, decor, bold, code))
	}
	if len(spans) == 0 {
		spans = append(spans, textSpan{text: text, fg: fg, decor: decor})
	}
	return spans
}

func parseInlineMarkdownPlainStyle(text, fg string) []textSpan {
	spans := parseInlineMarkdown(text, fg, "")
	for i := range spans {
		spans[i].fg = fg
		spans[i].decor = ""
	}
	return spans
}

func inlineSpan(text, fg, decor string, bold, code bool) textSpan {
	if code {
		fg = fgInlineCode
	}
	return textSpan{text: text, fg: fg, decor: inlineDecor(decor, bold)}
}

func wrapInlineMarkdown(text string, width int, fg, decor string) [][]textSpan {
	wrapped := wrapParagraph(text, width)
	spans := make([][]textSpan, 0, len(wrapped))
	for _, line := range wrapped {
		spans = append(spans, parseInlineMarkdown(line, fg, decor))
	}
	return spans
}

func inlineDecor(base string, bold bool) string {
	if bold || base == decorBold {
		return decorBold
	}
	return base
}

// ── Text wrapping ─────────────────────────────────────────────────────────────

// wrapText breaks text at word boundaries, preserving newlines.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var out []string
	for _, para := range strings.Split(text, "\n") {
		out = append(out, wrapParagraph(para, width)...)
	}
	return out
}

// wrapParagraph wraps a single line of text (no embedded newlines) at word
// boundaries. Words longer than width are hard-split.
func wrapParagraph(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var cur []rune

	flush := func() {
		if len(cur) > 0 {
			lines = append(lines, string(cur))
			cur = nil
		}
	}

	for _, word := range words {
		wr := []rune(word)

		// Word is too long to fit on any line – hard-split it.
		if len(wr) > width {
			flush()
			for len(wr) > 0 {
				take := width
				if take > len(wr) {
					take = len(wr)
				}
				lines = append(lines, string(wr[:take]))
				wr = wr[take:]
			}
			continue
		}

		switch {
		case len(cur) == 0:
			cur = append(cur, wr...)
		case len(cur)+1+len(wr) <= width:
			cur = append(cur, ' ')
			cur = append(cur, wr...)
		default:
			flush()
			cur = append(cur, wr...)
		}
	}
	flush()

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}
