package components

import (
	"strings"
	"time"

	"plums/internal/layout"
	"plums/internal/screen"
)

// ── Palette ───────────────────────────────────────────────────────────────────

const (
	fgContent    = "\x1b[38;2;200;198;212m"        // near-white for message body
	fgDimRule    = "\x1b[38;2;72;70;84m"           // very dim, for the ─── rule
	fgUserRole   = "\x1b[1m\x1b[38;2;80;220;120m"  // bold green  – "you"
	fgAiRole     = "\x1b[1m\x1b[38;2;100;190;255m" // bold blue  – "assistant"
	fgCursor     = "\x1b[38;2;160;220;255m"        // streaming cursor colour
	fgSystemRole = "\x1b[1m\x1b[38;2;220;160;50m"  // bold amber – system / error
	fgSystemBody = "\x1b[38;2;200;145;60m"         // dim amber for system body
)

// spinnerFrames is the Braille spinner sequence.
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

func currentSpinner() rune {
	frame := int(time.Now().UnixMilli()/80) % len(spinnerFrames)
	return spinnerFrames[frame]
}

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
	kind        lineKind
	text        string // for lineKindContent
	role        string // for lineKindHeader – the role label
	roleFg      string // for lineKindHeader – ANSI fg of the label
	contentFg   string // for lineKindContent – overrides default if non-empty
	showSpinner bool   // for lineKindHeader – animate the Braille spinner
}

// ── ChatLog component ─────────────────────────────────────────────────────────

type ChatLog struct {
	isDirty     bool
	messages    []ChatMessage
	aioutput    string
	isStreaming bool

	style  layout.Style
	parent layout.Component

	x, y int
	w, h int
}

func NewChatLog() *ChatLog {
	return &ChatLog{}
}

func (cl *ChatLog) SetMessages(msgs []ChatMessage) {
	cl.messages = msgs
	cl.isDirty = true
}

func (cl *ChatLog) SetAiOutput(s string) {
	cl.aioutput = s
	cl.isDirty = true
}

func (cl *ChatLog) SetStreaming(v bool) {
	cl.isStreaming = v
	cl.isDirty = true
}

func (cl *ChatLog) IsDirty() bool                { return cl.isDirty }
func (cl *ChatLog) MakeDirty()                   { cl.isDirty = true }
func (cl *ChatLog) ClearDirty()                  { cl.isDirty = false }
func (cl *ChatLog) GetStyle() layout.Style       { return cl.style }
func (cl *ChatLog) SetParent(p layout.Component) { cl.parent = p }
func (cl *ChatLog) SetStyle(s layout.Style)      { cl.style = s }

func (cl *ChatLog) Layout(x, y, w, h int) {
	cl.x, cl.y, cl.w, cl.h = x, y, w, h
}

// ── Render ────────────────────────────────────────────────────────────────────

func (cl *ChatLog) Render(s *screen.Screen) {
	bg := cl.style.GetBackground()
	if cl.parent != nil {
		bg = cl.parent.GetStyle().GetBackground()
	}

	// Build the full list of logical lines (may be taller than cl.h).
	lines := cl.buildLines()

	// Auto-scroll: always pin to the bottom so new tokens are visible.
	start := 0
	if len(lines) > cl.h {
		start = len(lines) - cl.h
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
		headerFg, bodyFg, roleLabel := cl.roleStyle(msg.Role)
		lines = append(lines, renderLine{
			kind:   lineKindHeader,
			role:   roleLabel,
			roleFg: headerFg,
		})
		for _, l := range wrapText(msg.Content, cl.contentWidth()) {
			lines = append(lines, renderLine{kind: lineKindContent, text: l, contentFg: bodyFg})
		}
	}

	// In-progress streaming response.
	if cl.aioutput != "" || cl.isStreaming {
		if len(cl.messages) > 0 {
			lines = append(lines, renderLine{kind: lineKindBlank})
		}
		lines = append(lines, renderLine{
			kind:        lineKindHeader,
			role:        "assistant",
			roleFg:      fgAiRole,
			showSpinner: cl.isStreaming,
		})
		if cl.aioutput != "" {
			content := cl.aioutput
			if cl.isStreaming {
				content += "▌"
			}
			for _, l := range wrapText(content, cl.contentWidth()) {
				lines = append(lines, renderLine{kind: lineKindContent, text: l, contentFg: fgContent})
			}
		}
	}

	return lines
}

func (cl *ChatLog) roleStyle(role string) (headerFg, bodyFg, label string) {
	switch role {
	case "user":
		return fgUserRole, fgContent, "you"
	case "ai":
		return fgAiRole, fgContent, "assistant"
	case "system":
		return fgSystemRole, fgSystemBody, "!"
	default:
		return fgContent, fgContent, role
	}
}

func (cl *ChatLog) contentWidth() int {
	w := cl.w - 2 // 2-space indent for body lines
	if w < 1 {
		w = 1
	}
	return w
}

// ── Per-row rendering ─────────────────────────────────────────────────────────

func (cl *ChatLog) renderLine(s *screen.Screen, y int, line renderLine, bg string) {
	switch line.kind {
	case lineKindBlank:
		cl.clearRow(s, y, bg)
	case lineKindHeader:
		cl.renderHeader(s, y, line.role, line.roleFg, bg, line.showSpinner)
	case lineKindContent:
		fg := fgContent
		if line.contentFg != "" {
			fg = line.contentFg
		}
		cl.renderContent(s, y, line.text, fg, bg)
	}
}

// renderHeader draws "role [spinner] ─────────────────" across the full row.
// The role name uses the coloured+bold fg; the separator uses the dim rule fg.
// When showSpinner is true a Braille spinner character is inserted after the label.
func (cl *ChatLog) renderHeader(s *screen.Screen, y int, role string, roleFg string, bg string, showSpinner bool) {
	x := cl.x

	// Role label.
	for _, r := range []rune(role) {
		if x >= cl.x+cl.w {
			break
		}
		s.Set(x, y, r, roleFg, bg, "")
		x++
	}

	// Braille spinner – only while streaming.
	if showSpinner {
		if x < cl.x+cl.w {
			s.Set(x, y, ' ', fgDimRule, bg, "")
			x++
		}
		if x < cl.x+cl.w {
			s.Set(x, y, currentSpinner(), roleFg, bg, "")
			x++
		}
	}

	// Space between label (or spinner) and rule.
	if x < cl.x+cl.w {
		s.Set(x, y, ' ', fgDimRule, bg, "")
		x++
	}

	// Horizontal rule filling the rest.
	for x < cl.x+cl.w {
		s.Set(x, y, '─', fgDimRule, bg, "")
		x++
	}
}

// renderContent draws "  <text><padding>" using the given foreground colour.
func (cl *ChatLog) renderContent(s *screen.Screen, y int, text string, fg string, bg string) {
	x := cl.x

	// 2-space indent.
	for i := 0; i < 2 && x < cl.x+cl.w; i++ {
		s.Set(x, y, ' ', fg, bg, "")
		x++
	}

	// Text runes. The trailing block cursor ▌ gets its own highlight colour.
	runes := []rune(text)
	for i, r := range runes {
		if x >= cl.x+cl.w {
			break
		}
		cellFg := fg
		if r == '▌' && i == len(runes)-1 {
			cellFg = fgCursor
		}
		s.Set(x, y, r, cellFg, bg, "")
		x++
	}

	// Fill remainder.
	for x < cl.x+cl.w {
		s.Set(x, y, ' ', fg, bg, "")
		x++
	}
}

func (cl *ChatLog) clearRow(s *screen.Screen, y int, bg string) {
	for x := cl.x; x < cl.x+cl.w; x++ {
		s.Set(x, y, ' ', fgContent, bg, "")
	}
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
