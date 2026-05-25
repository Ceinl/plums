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
	chatLog.SetScrollOffset(state.OutputScroll())
	return chatLog
}

func newGitDiffLog(state *State) *components.DiffLog {
	diffLog := components.NewDiffLog()
	diffLog.SetContent(state.GitDiff)
	diffLog.SetScrollOffset(state.OutputScroll())
	return diffLog
}

func newInfoTabs(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	div.SetStyle(style)

	tabs := components.NewInfoTabs()
	tabs.SetTabs([]components.InfoTab{
		{Label: "AI output", Active: state.InfoView == InfoViewAI},
		{Label: "Git diff", Active: state.InfoView == InfoViewGitDiff},
	})
	div.AppendChild(tabs)
	return div
}

func newInfoView(state *State) layout.Component {
	if state.InfoView == InfoViewGitDiff {
		return newGitDiffLog(state)
	}
	return newChatLog(state)
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

func newSplitStatusBar(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	style.SetForeground(100, 98, 112)
	div.SetStyle(style)

	bar := components.NewStatusBar()
	bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	bar.SetSession(state.SessionTitle)
	bar.SetModel(state.ModelProvider, state.ModelID)
	div.AppendChild(bar)
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

func newPalettePanel(state *State, w, h layout.Unit) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)
	div.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

	style := layout.Style{}
	style.SetBackground(32, 30, 40)
	style.SetForeground(220, 218, 230)
	div.SetStyle(style)

	popup := components.NewPopup()
	popup.SetPanel(true)
	popup.SetTitle(state.PaletteTitle())
	popup.SetItems(state.PaletteItems(), state.PaletteIndex)
	div.AppendChild(popup)
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
	if state.PopupOpen && (state.EffectiveLayout() != LayoutSplit || state.width < MinSplitLayoutWidth) {
		popup := components.NewPopup()
		popup.SetTitle(state.PaletteTitle())
		popup.SetItems(state.PaletteItems(), state.PaletteIndex)
		popup.Layout(0, 0, state.width, state.height)
		popup.Render(scr)
	}

	scr.Flush()
	if state.PopupOpen {
		return
	}
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
	if state.width < MinSplitLayoutWidth {
		return CreateNarrowSplitLayout(state)
	}

	// Left pane: editor, temporarily replaced by command palette when open.
	var leftDiv *components.Div
	if state.PopupOpen {
		leftDiv = newPalettePanel(
			state,
			layout.Unit{Type: layout.UnitPersent, Value: 50},
			layout.Unit{Type: layout.UnitPersent, Value: 100},
		)
	} else {
		leftDiv = newEditorDiv(
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
	}

	// 1-column accent separator
	sep := newVerticalSeparator()
	statusSep := components.NewSeparator()
	sep.AppendChild(statusSep)

	// Right pane: chat output (fills remaining space)
	rightDiv := newOutput()
	rightDiv.SetSize(
		layout.Unit{Type: layout.UnitGrow, Value: 1},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	rightDiv.AppendChild(newInfoTabs(state))
	rightDiv.AppendChild(newInfoView(state))
	rightDiv.AppendChild(newSplitStatusBar(state))

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

func CreateNarrowSplitLayout(state *State) *components.Div {
	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 50},
	)
	outputDiv.AppendChild(newChatLog(state))

	sepDiv := newHorizontalRule(state)

	inputDiv := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
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
