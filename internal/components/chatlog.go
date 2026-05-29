package components

import (
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/Ceinl/plums/internal/layout"
	"github.com/Ceinl/plums/internal/screen"
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
	blocks := parseMarkdownBlocks(content)
	var lines []renderLine

	for _, block := range blocks {
		if block.isCode {
			for _, spans := range wrapSpanLines(highlightCode(block.lang, block.text, fg), cl.contentWidth()) {
				lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg})
			}
			continue
		}

		lines = append(lines, cl.buildMarkdownLines(block.text, fg, bg)...)
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
	for _, para := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(para)
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
					lines = append(lines, renderLine{kind: lineKindContent, spans: spans, contentFg: fg, contentBg: bg})
				}
			}
			continue
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
	hasMarkup := inThinking || strings.Contains(text, "<think>") || strings.Contains(text, "</think>")
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
		openIdx := strings.Index(remaining, "<think>")
		closeIdx := strings.Index(remaining, "</think>")

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
			remaining = remaining[closeIdx+len("</think>"):]
			inThinking = false
			if !strings.HasPrefix(remaining, "<think>") && (len(spans) > 0 || mode != ThinkingVisibilityHidden) {
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
			remaining = remaining[closeIdx+len("</think>"):]
			continue
		}
		if openIdx > 0 {
			appendNormal(remaining[:openIdx])
		}
		remaining = remaining[openIdx+len("<think>"):]
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

type markdownBlock struct {
	isCode bool
	lang   string
	text   string
}

func parseMarkdownBlocks(content string) []markdownBlock {
	var blocks []markdownBlock
	var textLines []string
	var codeLines []string
	var codeLang string
	inCode := false

	flushText := func() {
		if len(textLines) == 0 {
			return
		}
		blocks = append(blocks, markdownBlock{text: strings.Join(textLines, "\n")})
		textLines = nil
	}
	flushCode := func() {
		blocks = append(blocks, markdownBlock{isCode: true, lang: codeLang, text: strings.Join(codeLines, "\n")})
		codeLines = nil
		codeLang = ""
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
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
			codeLines = append(codeLines, line)
		} else {
			textLines = append(textLines, line)
		}
	}

	if inCode {
		flushCode()
	} else {
		flushText()
	}

	return blocks
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
