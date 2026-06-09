package components

import (
	"strings"
)

// ── Table rendering ───────────────────────────────────────────────────────────

func isTableRow(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
}

func isTableSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return false
	}
	inner := trimmed[1 : len(trimmed)-1]
	for _, part := range strings.Split(inner, "|") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, r := range part {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
	}
	return strings.Contains(inner, "-")
}

func parseTableCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	inner := trimmed[1 : len(trimmed)-1]
	parts := strings.Split(inner, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func runeWidth(s string) int {
	return len([]rune(s))
}

func spansWidth(spans []textSpan) int {
	w := 0
	for _, s := range spans {
		w += len([]rune(s.text))
	}
	return w
}

func truncateSpans(spans []textSpan, maxWidth int, fg string) []textSpan {
	width := spansWidth(spans)
	if width <= maxWidth {
		return spans
	}
	if maxWidth <= 0 {
		return nil
	}
	if maxWidth <= 3 {
		return []textSpan{{text: strings.Repeat(".", maxWidth), fg: fg}}
	}

	result := make([]textSpan, 0, len(spans))
	remaining := maxWidth - 3
	for _, s := range spans {
		sw := len([]rune(s.text))
		if sw <= remaining {
			result = append(result, s)
			remaining -= sw
		} else {
			runes := []rune(s.text)
			if remaining > 0 {
				result = append(result, textSpan{text: string(runes[:remaining]), fg: s.fg, decor: s.decor})
			}
			break
		}
	}
	result = append(result, textSpan{text: "...", fg: fg})
	return result
}

func scaleColumnWidths(maxWidths []int, available int) []int {
	total := 0
	for _, w := range maxWidths {
		total += w
	}
	if total <= available {
		return maxWidths
	}

	result := make([]int, len(maxWidths))
	const minW = 3
	if len(maxWidths)*minW > available {
		for i := range result {
			result[i] = minW
		}
		return result
	}

	extra := available - len(maxWidths)*minW
	for i, w := range maxWidths {
		if total > 0 {
			result[i] = minW + int(float64(w)/float64(total)*float64(extra))
		} else {
			result[i] = minW
		}
	}

	used := 0
	for _, w := range result {
		used += w
	}
	for used < available {
		best := -1
		bestDiff := -1
		for i, w := range maxWidths {
			diff := w - result[i]
			if diff > bestDiff {
				bestDiff = diff
				best = i
			}
		}
		if best == -1 || bestDiff <= 0 {
			break
		}
		result[best]++
		used++
	}

	return result
}

func (cl *ChatLog) buildTableLines(rawRows []string, parsedRows [][]string, fg, bg string) []renderLine {
	width := cl.contentWidth()

	cols := 0
	for _, row := range parsedRows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return nil
	}

	overhead := (cols + 1) + 2*cols // pipes + spaces around cells
	if width <= overhead {
		return cl.buildTableLinesAsText(parsedRows, fg, bg)
	}

	isSep := make([]bool, len(rawRows))
	for i, raw := range rawRows {
		isSep[i] = isTableSeparator(raw)
	}

	maxWidths := make([]int, cols)
	for i, row := range parsedRows {
		if isSep[i] {
			continue
		}
		for ci, cell := range row {
			w := runeWidth(cell)
			if w > maxWidths[ci] {
				maxWidths[ci] = w
			}
		}
	}

	available := width - overhead
	colWidths := scaleColumnWidths(maxWidths, available)

	var lines []renderLine
	pipeSpan := textSpan{text: "|", fg: fgCodeFence}
	spaceSpan := textSpan{text: " ", fg: fg}

	for ri, row := range parsedRows {
		if isSep[ri] {
			var spans []textSpan
			spans = append(spans, pipeSpan)
			for ci := 0; ci < cols; ci++ {
				dashLen := colWidths[ci] + 2
				spans = append(spans, textSpan{text: strings.Repeat("─", dashLen), fg: fgDimRule})
				spans = append(spans, pipeSpan)
			}
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg})
			continue
		}

		var spans []textSpan
		spans = append(spans, pipeSpan)
		for ci := 0; ci < cols; ci++ {
			cell := ""
			if ci < len(row) {
				cell = row[ci]
			}
			cellSpans := parseInlineMarkdown(cell, fg, "")
			cellSpans = truncateSpans(cellSpans, colWidths[ci], fg)
			pad := colWidths[ci] - spansWidth(cellSpans)

			spans = append(spans, spaceSpan)
			spans = append(spans, cellSpans...)
			if pad > 0 {
				spans = append(spans, textSpan{text: strings.Repeat(" ", pad), fg: fg})
			}
			spans = append(spans, spaceSpan)
			spans = append(spans, pipeSpan)
		}
		lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg})
	}

	return lines
}

func (cl *ChatLog) buildTableLinesAsText(parsedRows [][]string, fg, bg string) []renderLine {
	var lines []renderLine
	for _, row := range parsedRows {
		text := strings.Join(row, " | ")
		for _, spans := range wrapInlineMarkdown(text, cl.contentWidth(), fg, "") {
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg})
		}
	}
	return lines
}
