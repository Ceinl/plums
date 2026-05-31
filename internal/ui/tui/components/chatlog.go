package components

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// ── Palette ───────────────────────────────────────────────────────────────────

const (
	listIndent    = 2
	fgContent     = "\x1b[38;2;200;198;212m" // near-white for message body
	fgDimRule     = "\x1b[38;2;72;70;84m"    // very dim, for the system rule
	fgCursor      = "\x1b[38;2;160;220;255m" // streaming cursor colour
	fgHeading     = "\x1b[38;2;238;234;248m" // brighter section headings
	fgThinking    = "\x1b[38;2;132;130;145m" // muted thinking / reasoning trace
	fgInlineCode  = "\x1b[38;2;245;190;120m"
	fgUserAccent  = "\x1b[38;2;247;184;90m"
	fgSystemRole  = "\x1b[1m\x1b[38;2;220;160;50m" // bold amber – system / error
	fgSystemBody  = "\x1b[38;2;200;145;60m"        // dim amber for system body
	fgCodeFence   = "\x1b[38;2;120;118;140m"       // dim purple for code fences
	fgListMarker  = "\x1b[38;2;155;188;255m"       // cool accent for bullets / numbers
	bgUserMessage = "\x1b[48;2;34;32;42m"          // lighter panel for user prompts
	decorBold     = "\x1b[1m"

	// selection highlight colours
	selFg = "\x1b[38;2;22;20;27m"
	selBg = "\x1b[48;2;200;198;212m"

	fgToolCall      = "\x1b[38;2;160;230;180m" // soft green for tool calls
	fgToolOutput    = "\x1b[38;2;160;180;220m" // soft blue-gray for tool output
	fgToolIndicator = "\x1b[38;2;245;190;120m" // soft yellow-orange for the diamond indicator
)

// ── Types ─────────────────────────────────────────────────────────────────────

type ChatMessage struct {
	Role    string
	Content string
}

// lineKind tags a pre-rendered logical line.
type lineKind int

const (
	lineKindBlank   lineKind = iota
	lineKindHeader           // role name + ─── rule
	lineKindContent          // indented body text
)

// renderLine is one terminal row worth of content.
type renderLine struct {
	kind      lineKind
	text      string // for lineKindContent
	spans     []textSpan
	role      string // for lineKindHeader – the role label
	roleFg    string // for lineKindHeader – ANSI fg of the label
	contentFg string // for lineKindContent – overrides default if non-empty
	contentBg string // for lineKindContent – overrides parent background if non-empty
	accentFg  string
}

type textSpan struct {
	text  string
	fg    string
	decor string
}

type spanLine struct {
	spans []textSpan
}

type ThinkingVisibility int

const (
	ThinkingVisibilityFull ThinkingVisibility = iota
	ThinkingVisibilityTitle
	ThinkingVisibilityHidden
)

// ── ChatLog component ─────────────────────────────────────────────────────────

type ChatLog struct {
	isDirty      bool
	messages     []ChatMessage
	aioutput     string
	isStreaming  bool
	scrollOffset int
	onMaxScroll  func(int)
	linesCached  bool
	cachedWidth  int
	cachedLines  []renderLine
	thinkingMode ThinkingVisibility

	style  layout.Style
	parent layout.Component

	x, y int
	w, h int

	// mouse selection
	selActive            bool
	selStartX, selStartY int
	selEndX, selEndY     int
}

func NewChatLog() *ChatLog {
	return &ChatLog{}
}

func (cl *ChatLog) SetMessages(msgs []ChatMessage) {
	if chatMessagesEqual(cl.messages, msgs) {
		return
	}
	cl.messages = msgs
	cl.invalidateLines()
	cl.ClearSelection()
	cl.isDirty = true
}

func (cl *ChatLog) SetAiOutput(s string) {
	if cl.aioutput == s {
		return
	}
	cl.aioutput = s
	cl.invalidateLines()
	cl.ClearSelection()
	cl.isDirty = true
}

func (cl *ChatLog) SetStreaming(v bool) {
	if cl.isStreaming == v {
		return
	}
	cl.isStreaming = v
	cl.invalidateLines()
	cl.ClearSelection()
	cl.isDirty = true
}

func (cl *ChatLog) SetThinkingVisibility(v ThinkingVisibility) {
	if cl.thinkingMode == v {
		return
	}
	cl.thinkingMode = v
	cl.invalidateLines()
	cl.ClearSelection()
	cl.isDirty = true
}

func (cl *ChatLog) SetScrollOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	if cl.scrollOffset == offset {
		return
	}
	cl.scrollOffset = offset
	cl.ClearSelection()
	cl.isDirty = true
}

func chatMessagesEqual(a, b []ChatMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (cl *ChatLog) SetMaxScrollObserver(fn func(int)) {
	cl.onMaxScroll = fn
}

func (cl *ChatLog) IsDirty() bool                { return cl.isDirty }
func (cl *ChatLog) MakeDirty()                   { cl.isDirty = true }
func (cl *ChatLog) ClearDirty()                  { cl.isDirty = false }
func (cl *ChatLog) GetStyle() layout.Style       { return cl.style }
func (cl *ChatLog) SetParent(p layout.Component) { cl.parent = p }
func (cl *ChatLog) SetStyle(s layout.Style)      { cl.style = s }

func (cl *ChatLog) Layout(x, y, w, h int) {
	if cl.w != w {
		cl.invalidateLines()
	}
	cl.x, cl.y, cl.w, cl.h = x, y, w, h
}

func (cl *ChatLog) MouseDown(x, y int) bool {
	if x < cl.x || x >= cl.x+cl.w || y < cl.y || y >= cl.y+cl.h {
		return false
	}
	cl.selActive = true
	cl.selStartX, cl.selStartY = x, y
	cl.selEndX, cl.selEndY = x, y
	cl.isDirty = true
	return true
}

func (cl *ChatLog) MouseDrag(x, y int) {
	if !cl.selActive {
		return
	}
	cl.selEndX, cl.selEndY = x, y
	cl.isDirty = true
}

func (cl *ChatLog) MouseUp(x, y int) string {
	if !cl.selActive {
		return ""
	}
	cl.selEndX, cl.selEndY = x, y
	text := cl.extractSelection()
	cl.selActive = false
	cl.isDirty = true
	return text
}

func (cl *ChatLog) ClearSelection() {
	cl.selActive = false
	cl.isDirty = true
}

func (cl *ChatLog) rowSelectionRange(screenY int) (startX, endX int, ok bool) {
	if !cl.selActive {
		return 0, 0, false
	}
	sy1, sy2 := cl.selStartY, cl.selEndY
	if sy1 > sy2 {
		sy1, sy2 = sy2, sy1
	}
	if screenY < sy1 || screenY > sy2 {
		return 0, 0, false
	}

	sx1, sx2 := cl.selStartX, cl.selEndX
	if sy1 != sy2 {
		if cl.selStartY <= cl.selEndY {
			if screenY == cl.selStartY {
				return sx1, cl.x + cl.w, true
			}
			if screenY == cl.selEndY {
				return cl.x, sx2, true
			}
			return cl.x, cl.x + cl.w, true
		}
		// dragged upward
		if screenY == cl.selEndY {
			return sx2, cl.x + cl.w, true
		}
		if screenY == cl.selStartY {
			return cl.x, sx1, true
		}
		return cl.x, cl.x + cl.w, true
	}
	if sx1 > sx2 {
		sx1, sx2 = sx2, sx1
	}
	return sx1, sx2, true
}

func (cl *ChatLog) extractSelection() string {
	if !cl.selActive {
		return ""
	}

	lines := cl.lines()
	maxScroll := len(lines) - cl.h
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollOffset := cl.scrollOffset
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	start := 0
	if len(lines) > cl.h {
		maxStart := len(lines) - cl.h
		start = maxStart - scrollOffset
		if start < 0 {
			start = 0
		}
	}

	var out []string
	for row := 0; row < cl.h; row++ {
		y := cl.y + row
		idx := start + row
		if idx >= len(lines) {
			continue
		}

		rx1, rx2, ok := cl.rowSelectionRange(y)
		if !ok {
			continue
		}

		line := lines[idx]
		if line.kind == lineKindBlank {
			out = append(out, "")
			continue
		}

		var runes []rune
		curX := cl.x
		for _, span := range line.spans {
			for _, r := range span.text {
				if curX >= rx1 && curX < rx2 {
					runes = append(runes, r)
				}
				curX++
			}
		}
		if len(runes) > 0 {
			out = append(out, string(runes))
		}
	}

	return strings.Join(out, "\n")
}

func (cl *ChatLog) MaxScrollOffset() int {
	maxOffset := len(cl.lines()) - cl.h
	if maxOffset < 0 {
		return 0
	}
	return maxOffset
}

func (cl *ChatLog) invalidateLines() {
	cl.linesCached = false
	cl.cachedLines = nil
}

func (cl *ChatLog) lines() []renderLine {
	width := cl.contentWidth()
	if cl.linesCached && cl.cachedWidth == width {
		return cl.cachedLines
	}
	cl.cachedLines = cl.buildLines()
	cl.cachedWidth = width
	cl.linesCached = true
	return cl.cachedLines
}

// ── Render ────────────────────────────────────────────────────────────────────

func (cl *ChatLog) Render(s *screen.Screen) {
	bg := cl.style.GetBackground()
	if cl.parent != nil {
		bg = cl.parent.GetStyle().GetBackground()
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

// buildLines converts all messages (and in-progress AI output) into a flat
// slice of renderLine values that represent one terminal row each.
func (cl *ChatLog) buildLines() []renderLine {
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

	// In-progress streaming response.
	if cl.aioutput != "" || cl.isStreaming {
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
	}

	return lines
}

func (cl *ChatLog) buildContentLines(content, fg, bg string) []renderLine {
	blocks := parseChatBlocks(content)
	var lines []renderLine

	for _, block := range blocks {
		switch block.kind {
		case blockKindCode:
			bgCodeBlock := "\x1b[48;2;24;22;30m"
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

		case blockKindToolCall:
			displayText := truncateToolSummary(toolCallSummary(block.toolName, block.text), cl.contentWidth()-4)

			spans := []textSpan{
				{text: "◆ ", fg: fgToolIndicator},
				{text: displayText, fg: fgToolCall},
			}
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fgToolCall, contentBg: bg})

		case blockKindToolOutput:
			outputCompact := compactToOneLine(block.text)
			displayText := truncateToolSummary("response: "+outputCompact, cl.contentWidth()-4)

			spans := []textSpan{
				{text: "◇ ", fg: fgToolIndicator},
				{text: displayText, fg: fgToolOutput},
			}
			lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fgToolOutput, contentBg: bg})

		case blockKindText:
			lines = append(lines, cl.buildMarkdownLines(block.text, fg, bg)...)
		}
	}

	return lines
}

func (cl *ChatLog) roleStyle(role string) (bodyFg, bodyBg string) {
	switch role {
	case "user":
		return fgContent, bgUserMessage
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

// ── Markdown/code highlighting ───────────────────────────────────────────────

type blockKind int

const (
	blockKindText blockKind = iota
	blockKindCode
	blockKindToolCall
	blockKindToolOutput
)

type chatBlock struct {
	kind     blockKind
	toolName string
	lang     string
	text     string
}

func parseChatBlocks(content string) []chatBlock {
	var blocks []chatBlock
	var currentText []string
	var currentCode []string
	var currentTool []string

	inCode := false
	inToolCall := false
	inToolOutput := false
	toolName := ""
	codeLang := ""

	flushText := func() {
		if len(currentText) == 0 {
			return
		}
		blocks = append(blocks, chatBlock{
			kind: blockKindText,
			text: strings.Join(currentText, "\n"),
		})
		currentText = nil
	}

	flushCode := func() {
		blocks = append(blocks, chatBlock{
			kind: blockKindCode,
			lang: codeLang,
			text: strings.Join(currentCode, "\n"),
		})
		currentCode = nil
		codeLang = ""
	}

	flushToolCall := func() {
		blocks = append(blocks, chatBlock{
			kind:     blockKindToolCall,
			toolName: toolName,
			text:     strings.Join(currentTool, "\n"),
		})
		currentTool = nil
		toolName = ""
	}

	flushToolOutput := func() {
		blocks = append(blocks, chatBlock{
			kind: blockKindToolOutput,
			text: strings.Join(currentTool, "\n"),
		})
		currentTool = nil
	}

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 1. Code Block boundary
		if strings.HasPrefix(trimmed, "```") && !inToolCall && !inToolOutput {
			if inCode {
				flushCode()
				inCode = false
			} else {
				flushText()
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				inCode = true
			}
			continue
		}

		if inCode {
			currentCode = append(currentCode, line)
			continue
		}

		if name, input, ok := parsePlainToolCall(trimmed); ok {
			flushText()
			blocks = append(blocks, chatBlock{
				kind:     blockKindToolCall,
				toolName: name,
				text:     input,
			})
			continue
		}

		// 2. Tool Call / Output boundary checks
		if strings.Contains(trimmed, "<tool_call") {
			flushText()

			// Extract tool name if present: name="xyz" or name='xyz'
			name := ""
			if idx := strings.Index(trimmed, "name=\""); idx != -1 {
				sub := trimmed[idx+6:]
				if endIdx := strings.Index(sub, "\""); endIdx != -1 {
					name = sub[:endIdx]
				}
			} else if idx := strings.Index(trimmed, "name='"); idx != -1 {
				sub := trimmed[idx+6:]
				if endIdx := strings.Index(sub, "'"); endIdx != -1 {
					name = sub[:endIdx]
				}
			}

			if strings.Contains(trimmed, "</tool_call>") {
				contentInside := extractTagContent(trimmed, "<tool_call", "</tool_call>")
				blocks = append(blocks, chatBlock{
					kind:     blockKindToolCall,
					toolName: name,
					text:     contentInside,
				})
			} else {
				inToolCall = true
				toolName = name
				if tagEnd := strings.Index(line, ">"); tagEnd != -1 && tagEnd+1 < len(line) {
					rem := strings.TrimSpace(line[tagEnd+1:])
					if rem != "" {
						currentTool = append(currentTool, rem)
					}
				}
			}
			continue
		}

		if inToolCall {
			if strings.Contains(trimmed, "</tool_call>") {
				if idx := strings.Index(line, "</tool_call>"); idx != -1 {
					before := strings.TrimSpace(line[:idx])
					if before != "" {
						currentTool = append(currentTool, before)
					}
				}
				flushToolCall()
				inToolCall = false
			} else {
				currentTool = append(currentTool, line)
			}
			continue
		}

		isOutputStart := strings.Contains(trimmed, "<tool_output") || strings.Contains(trimmed, "<tool_response")
		if isOutputStart {
			flushText()

			closingTag := "</tool_output>"
			if strings.Contains(trimmed, "<tool_response") {
				closingTag = "</tool_response>"
			}

			if strings.Contains(trimmed, closingTag) {
				contentInside := extractTagContent(trimmed, "<tool_output", closingTag)
				if contentInside == "" && closingTag == "</tool_response>" {
					contentInside = extractTagContent(trimmed, "<tool_response", closingTag)
				}
				blocks = append(blocks, chatBlock{
					kind: blockKindToolOutput,
					text: contentInside,
				})
			} else {
				inToolOutput = true
				if tagEnd := strings.Index(line, ">"); tagEnd != -1 && tagEnd+1 < len(line) {
					rem := strings.TrimSpace(line[tagEnd+1:])
					if rem != "" {
						currentTool = append(currentTool, rem)
					}
				}
			}
			continue
		}

		if inToolOutput {
			hasClosingOutput := strings.Contains(trimmed, "</tool_output>")
			hasClosingResponse := strings.Contains(trimmed, "</tool_response>")
			if hasClosingOutput || hasClosingResponse {
				closingTag := "</tool_output>"
				if hasClosingResponse {
					closingTag = "</tool_response>"
				}
				if idx := strings.Index(line, closingTag); idx != -1 {
					before := strings.TrimSpace(line[:idx])
					if before != "" {
						currentTool = append(currentTool, before)
					}
				}
				flushToolOutput()
				inToolOutput = false
			} else {
				currentTool = append(currentTool, line)
			}
			continue
		}

		// 3. Normal text line
		currentText = append(currentText, line)
	}

	if inCode {
		flushCode()
	} else if inToolCall {
		flushToolCall()
	} else if inToolOutput {
		flushToolOutput()
	} else {
		flushText()
	}

	return blocks
}

func extractTagContent(s, tagOpen, tagClose string) string {
	idxOpen := strings.Index(s, tagOpen)
	if idxOpen == -1 {
		return ""
	}
	sub := s[idxOpen:]
	idxEndOpen := strings.Index(sub, ">")
	if idxEndOpen == -1 {
		return ""
	}
	start := idxOpen + idxEndOpen + 1

	idxClose := strings.Index(s, tagClose)
	if idxClose == -1 || idxClose <= start {
		return strings.TrimSpace(s[start:])
	}
	return strings.TrimSpace(s[start:idxClose])
}

func compactToOneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	words := strings.Fields(s)
	return strings.Join(words, " ")
}

func parsePlainToolCall(line string) (name, input string, ok bool) {
	const prefix = "Called the "
	const marker = " tool with the following input:"
	if !strings.HasPrefix(line, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, prefix)
	idx := strings.Index(rest, marker)
	if idx == -1 {
		return "", "", false
	}
	name = strings.TrimSpace(rest[:idx])
	input = strings.TrimSpace(rest[idx+len(marker):])
	if name == "" {
		return "", "", false
	}
	return name, input, true
}

func toolCallSummary(name, input string) string {
	if name == "" {
		name = "tool"
	}
	input = compactToOneLine(input)
	if input == "" {
		return "Called " + name
	}
	return "Called " + name + " with " + input
}

func truncateToolSummary(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func highlightCode(lang, code, fallbackFg string) [][]textSpan {
	lexer := lexers.Get(lang)
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		return plainCodeLines(code, fallbackFg)
	}
	lexer = chroma.Coalesce(lexer)

	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return plainCodeLines(code, fallbackFg)
	}

	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	lines := [][]textSpan{{}}
	for token := iterator(); token != chroma.EOF; token = iterator() {
		entry := style.Get(token.Type)
		fg := chromaColourToANSI(entry.Colour)
		if fg == "" {
			fg = fallbackFg
		}
		decor := ""
		if entry.Bold == chroma.Yes {
			decor = "\x1b[1m"
		}

		parts := strings.Split(token.Value, "\n")
		for i, part := range parts {
			if i > 0 {
				lines = append(lines, []textSpan{})
			}
			if part != "" {
				last := len(lines) - 1
				lines[last] = append(lines[last], textSpan{text: part, fg: fg, decor: decor})
			}
		}
	}

	return lines
}

func chromaColourToANSI(c chroma.Colour) string {
	if !c.IsSet() {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.Red(), c.Green(), c.Blue())
}

func plainCodeLines(code, fg string) [][]textSpan {
	parts := strings.Split(code, "\n")
	lines := make([][]textSpan, len(parts))
	for i, part := range parts {
		if part != "" {
			lines[i] = []textSpan{{text: part, fg: fg}}
		}
	}
	return lines
}

func wrapSpanLines(lines [][]textSpan, width int) [][]textSpan {
	if width <= 0 {
		return lines
	}

	var out [][]textSpan
	for _, line := range lines {
		out = append(out, wrapSpansHard(line, width)...)
	}
	return out
}

func wrapSpansHard(spans []textSpan, width int) [][]textSpan {
	if len(spans) == 0 {
		return [][]textSpan{{}}
	}

	var out [][]textSpan
	var cur []textSpan
	curWidth := 0
	appendRune := func(r rune, span textSpan) {
		if curWidth >= width {
			out = append(out, cur)
			cur = nil
			curWidth = 0
		}
		if len(cur) > 0 && cur[len(cur)-1].fg == span.fg && cur[len(cur)-1].decor == span.decor {
			cur[len(cur)-1].text += string(r)
		} else {
			cur = append(cur, textSpan{text: string(r), fg: span.fg, decor: span.decor})
		}
		curWidth++
	}

	for _, span := range spans {
		for _, r := range span.text {
			appendRune(r, span)
		}
	}
	if cur != nil {
		out = append(out, cur)
	}
	if len(out) == 0 {
		out = append(out, []textSpan{})
	}
	return out
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
