package components

import (
	"strings"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// ── Palette ───────────────────────────────────────────────────────────────────

const (
	listIndent = 2
	decorBold  = "\x1b[1m"
)

var (
	fgCursor     = theme.ChatCursor.Fg() // streaming cursor colour
	fgInlineCode = theme.ChatInlineCode.Fg()
	fgCodeFence  = theme.ChatCodeFence.Fg()  // dim purple for code fences
	fgListMarker = theme.ChatListMarker.Fg() // cool accent for bullets / numbers

	fgToolCall      = theme.ToolCall.Fg()      // dim green for the tool name
	fgToolArg       = theme.TextMuted.Fg()     // primary argument next to the tool name
	fgToolOutput    = theme.TextFaint.Fg()     // output should recede behind prose
	fgToolMarker    = theme.TextDim.Fg()       // the ⎿ connector under a tool call
	fgToolIndicator = theme.ToolIndicator.Fg() // dim amber for the ● indicator

	fgContent     = theme.TextSoft.Fg()               // near-white for message body
	fgDimRule     = theme.TextDim.Fg()                // very dim, for the system rule
	fgHeading     = theme.TextBright.Fg()             // brighter section headings
	fgThinking    = theme.TextMuted.Fg()              // muted thinking / reasoning trace
	fgUserAccent  = theme.Accent.Fg()                 // user message accent
	fgSystemRole  = decorBold + theme.AccentBold.Fg() // bold amber – system / error
	fgSystemBody  = theme.AccentSoft.Fg()             // dim amber for system body
	bgUserMessage = theme.BgRaised.Bg()               // lighter panel for user prompts

	// selection highlight colours, shared with the editor and input box
	selFg = theme.SelectionFg.Fg()
	selBg = theme.SelectionBg.Bg()
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

type ToolCallVisibility int

const (
	ToolCallVisibilityFull ToolCallVisibility = iota
	ToolCallVisibilityCollapse
	ToolCallVisibilityHidden
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
	toolCallMode ToolCallVisibility

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

func (cl *ChatLog) SetToolCallVisibility(v ToolCallVisibility) {
	if cl.toolCallMode == v {
		return
	}
	cl.toolCallMode = v
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

func (cl *ChatLog) IsDirty() bool { return cl.isDirty }

func (cl *ChatLog) MakeDirty() { cl.isDirty = true }

func (cl *ChatLog) ClearDirty() { cl.isDirty = false }

func (cl *ChatLog) GetStyle() layout.Style { return cl.style }

func (cl *ChatLog) SetParent(p layout.Component) { cl.parent = p }

func (cl *ChatLog) SetStyle(s layout.Style) { cl.style = s }

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
