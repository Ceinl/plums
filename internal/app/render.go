package app

import (
	"plums/internal/components"
	"plums/internal/layout"
	"plums/internal/screen"
)

var scr *screen.Screen

// ── Colour palette ────────────────────────────────────────────────────────────
//
// Default / fullscreen layout
//   bgOutput  – the chat / output area
//   bgEditor  – the editor area
//
// Split layout
//   bgSplitEditor – left pane (slightly lighter, warm)
//   bgSplitOutput – right pane (darker, more contrast)
//   bgSeparator   – 1-column vertical divider

// ── Factory helpers ───────────────────────────────────────────────────────────

func newOutput() *components.Div {
	outputDiv := components.NewDiv()
	outputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
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
	chatLog.SetStreaming(state.IsStreaming())
	return chatLog
}

func newHorizontalRule(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	sep := components.NewSeparator()
	sep.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	div.AppendChild(sep)
	return div
}

func newVerticalSeparator() *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPx, Value: 1},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	return div
}

func newTextDiv(content string, w, h layout.Unit, bgR, bgG, bgB uint8) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(bgR, bgG, bgB)
	style.SetForeground(100, 98, 112)
	div.SetStyle(style)

	text := components.NewText()
	text.SetContent(content)
	div.AppendChild(text)
	return div
}

func newEditorDiv(ed *components.Editor, w, h layout.Unit) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(32, 30, 40)
	style.SetForeground(220, 218, 230)
	div.SetStyle(style)
	div.AppendChild(ed)
	return div
}

// ── Main render entry point ───────────────────────────────────────────────────

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

	switch state.EffectiveLayout() {
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

// ── Layout builders ───────────────────────────────────────────────────────────

func CreateDefaultLayout(state *State) *components.Div {
	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	outputDiv.AppendChild(newChatLog(state))

	sepDiv := newHorizontalRule(state)

	inputDiv := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 5},
	)
	inputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

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
	// Left pane: editor (50 %)
	leftDiv := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 50},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	leftDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

	// 1-column accent separator
	sep := newVerticalSeparator()
	statusSep := components.NewSeparator()
	statusSep.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	sep.AppendChild(statusSep)

	// Right pane: chat output (fills remaining space)
	rightDiv := newOutput()
	rightDiv.SetSize(
		layout.Unit{Type: layout.UnitGrow, Value: 1},
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
	root.AppendChild(sep)
	root.AppendChild(rightDiv)
	return root
}

func CreateFullscreenLayout(state *State) *components.Div {
	div := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	div.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	return div
}
