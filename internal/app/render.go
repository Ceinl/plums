package app

import (
	"strings"

	"plums/internal/components"
	"plums/internal/layout"
	"plums/internal/screen"
)

var scr *screen.Screen

func newOutput() *components.Div {
	outputDiv := components.NewDiv()
	outputDiv.SetPadding(
		layout.Padding{
			Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
			Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
			Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
			Bottom: layout.Unit{Type: layout.UnitPx, Value: 1}})

	style := layout.Style{}
	style.SetBackground(25, 23, 29)
	outputDiv.SetStyle(style)
	return outputDiv
}

func newChatLog(state *State) *components.ChatLog {
	chatLog := components.NewChatLog()
	msgs := make([]components.ChatMessage, len(state.messages))
	for i, m := range state.messages {
		msgs[i] = components.ChatMessage{Role: m.Role, Content: m.Content}
	}
	chatLog.SetMessages(msgs)
	chatLog.SetAiOutput(state.aioutput)
	return chatLog
}

func newSeparatorDiv(state *State) *components.Div {
	return newTextDiv(strings.Repeat("\u2500", state.width),
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
		40, 38, 45,
	)
}

func newTextDiv(content string, w, h layout.Unit, bgR, bgG, bgB uint8) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(bgR, bgG, bgB)
	style.SetForeground(150, 148, 155)
	div.SetStyle(style)

	text := components.NewText()
	text.SetContent(content)
	div.AppendChild(text)
	return div
}

func newTextBoxDiv(ed *components.Editor, w, h layout.Unit, bgR, bgG, bgB uint8) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(bgR, bgG, bgB)
	style.SetForeground(220, 220, 220)
	div.SetStyle(style)
	div.AppendChild(ed)
	return div
}

func Render(state *State) {
	if scr == nil || scr.Width() != state.width || scr.Height() != state.height {
		scr = screen.NewScreen(state.width, state.height)
	}
	scr.Clear()

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)

	switch state.Layout {
	case LayoutDefault:
		root.AppendChild(CreateDefaultLayout(state))
	case LayoutSplit:
		root.AppendChild(CreateSplitLayout(state))
	case LayoutFullscreen:
		root.AppendChild(CreateFullscreenLayout(state))
	}

	root.Layout(0, 0, state.width, state.height)
	root.Render(scr)

	scr.Flush()
	cx, cy := state.Editor.CursorScreenPos()
	scr.SetCursor(cx, cy)
	scr.ShowCursor()
}

func CreateDefaultLayout(state *State) *components.Div {
	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	outputDiv.AppendChild(newChatLog(state))

	sepDiv := newSeparatorDiv(state)

	inputDiv := newTextBoxDiv(state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 5},
		30, 28, 35,
	)

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	root.AppendChild(outputDiv)
	root.AppendChild(sepDiv)
	root.AppendChild(inputDiv)
	return root
}

func CreateSplitLayout(state *State) *components.Div {
	leftDiv := newTextBoxDiv(state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 50},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		30, 28, 35,
	)
	leftDiv.SetPadding(
		layout.Padding{
			Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
			Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
			Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
			Bottom: layout.Unit{Type: layout.UnitPx, Value: 1}})

	rightDiv := newOutput()
	rightDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 50},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	rightDiv.AppendChild(newChatLog(state))

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	root.SetDirection(layout.Row)
	root.AppendChild(leftDiv)
	root.AppendChild(rightDiv)
	return root
}

func CreateFullscreenLayout(state *State) *components.Div {
	return newTextBoxDiv(state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		30, 28, 35,
	)
}
