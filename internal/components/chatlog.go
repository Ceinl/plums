package components

import (
	"plums/internal/layout"
	"plums/internal/screen"
)

type ChatMessage struct {
	Role    string
	Content string
}

type ChatLog struct {
	isDirty  bool
	messages []ChatMessage
	aioutput string
	prefixW  int

	style  layout.Style
	parent layout.Component

	x, y int
	w, h int

	userStyle layout.Style
	aiStyle   layout.Style
}

func NewChatLog() *ChatLog {
	userStyle := layout.Style{}
	userStyle.SetForeground(80, 220, 120)
	aiStyle := layout.Style{}
	aiStyle.SetForeground(100, 190, 255)
	aiStyle.SetBackground(30, 28, 35)
	return &ChatLog{
		prefixW:  1,
		userStyle: userStyle,
		aiStyle:   aiStyle,
	}
}

func (cl *ChatLog) SetMessages(msgs []ChatMessage) {
	cl.messages = msgs
	cl.isDirty = true
}

func (cl *ChatLog) SetAiOutput(s string) {
	cl.aioutput = s
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

func (cl *ChatLog) Render(s *screen.Screen) {
	bg := cl.style.GetBackground()
	if cl.parent != nil {
		bg = cl.parent.GetStyle().GetBackground()
	}

	cy := cl.y
	maxW := cl.w - cl.prefixW - 2

	for _, msg := range cl.messages {
		if cy >= cl.y+cl.h {
			break
		}

		var lineBg, decor string
		var rightAlign bool
		switch msg.Role {
		case "user":
			lineBg = bg
			decor = cl.getUserDecor()
			rightAlign = true
		case "ai":
			lineBg = cl.aiStyle.GetBackground()
			decor = cl.getAiDecor()
			rightAlign = false
		default:
			lineBg = bg
			decor = ""
			rightAlign = false
		}

		for _, line := range wrapRunes(msg.Content, maxW) {
			if cy >= cl.y+cl.h {
				break
			}
			cl.drawLine(s, cy, line, lineBg, rightAlign, decor)
			cy++
		}
		cy++
	}

	if cl.aioutput != "" && cy < cl.y+cl.h {
		for _, line := range wrapRunes(cl.aioutput, maxW) {
			if cy >= cl.y+cl.h {
				break
			}
			cl.drawLine(s, cy, line, cl.aiStyle.GetBackground(), false, cl.getAiDecor())
			cy++
		}
	}

	for cy < cl.y+cl.h {
		for x := cl.x; x < cl.x+cl.w; x++ {
			s.Set(x, cy, ' ', "", bg, "")
		}
		cy++
	}
}

func (cl *ChatLog) drawLine(s *screen.Screen, cy int, text string, bg string, rightAlign bool, decor string) {
	runes := []rune(text)
	contentW := 2 + len(runes)
	if contentW > cl.w {
		contentW = cl.w
	}

	startX := cl.x
	if rightAlign {
		startX = cl.x + cl.w - contentW
	}

	x := cl.x
	for ; x < startX; x++ {
		s.Set(x, cy, ' ', "", bg, "")
	}
	if x < cl.x+cl.w {
		s.Set(x, cy, '\u2502', "", bg, decor)
		x++
	}
	if x < cl.x+cl.w {
		s.Set(x, cy, ' ', "", bg, "")
		x++
	}
	for _, r := range runes {
		if x >= cl.x+cl.w {
			break
		}
		s.Set(x, cy, r, "", bg, "")
		x++
	}
	for x < cl.x+cl.w {
		s.Set(x, cy, ' ', "", bg, "")
		x++
	}
}

func (cl *ChatLog) getUserDecor() string {
	return "\x1b[1m" + cl.userStyle.GetForeground()
}

func (cl *ChatLog) getAiDecor() string {
	return "\x1b[1m" + cl.aiStyle.GetForeground()
}

func wrapRunes(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		if len(runes) <= width {
			lines = append(lines, string(runes))
			break
		}
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	return lines
}
