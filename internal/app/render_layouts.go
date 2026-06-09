package app

import (
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

// ── Colour palette ────────────────────────────────────────────────────────────
//
// Chat layout
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
	chatLog := state.ChatLog()
	msgs := make([]components.ChatMessage, len(state.messages))
	for i, m := range state.messages {
		msgs[i] = components.ChatMessage{Role: m.Role, Content: m.Content}
	}
	chatLog.SetMessages(msgs)
	chatLog.SetAiOutput(state.aioutput)
	chatLog.SetStreaming(state.IsStreaming())
	chatLog.SetThinkingVisibility(state.ThinkingMode)
	chatLog.SetScrollOffset(state.OutputScroll())
	return chatLog
}

func newGitDiffLog(state *State) *components.DiffLog {
	diffLog := state.DiffLog()
	diffLog.SetContent(state.GitDiff)
	diffLog.SetScrollOffset(state.OutputScroll())
	return diffLog
}

func newInfoTabs(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
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

func newSessions(state *State, orientation components.SessionsOrientation) *components.Sessions {
	sessions := state.Sessions()
	if orientation == components.SessionsHorizontal {
		sessions = state.SessionsHorizontal()
	}
	sessions.SetOrientation(orientation)
	items := make([]components.SessionItem, 0, len(state.SessionItems)+1)
	foundCurrent := false
	for _, item := range state.SessionItems {
		current := item.Current || item.ID == state.SessionID
		if current {
			foundCurrent = true
		}
		items = append(items, components.SessionItem{
			ID:        item.ID,
			Title:     item.Title,
			Directory: item.Directory,
			Updated:   item.Updated,
			Current:   current,
		})
	}
	if !foundCurrent && (state.SessionID != "" || state.SessionTitle != "") {
		items = append([]components.SessionItem{{
			ID:      state.SessionID,
			Title:   state.SessionTitle,
			Current: true,
		}}, items...)
	}
	sessions.SetItems(items)
	return sessions
}

func newInfoView(state *State) layout.Component {
	if state.InfoView == InfoViewGitDiff {
		return newGitDiffLog(state)
	}
	return newChatLog(state)
}

func newFullscreenView(state *State) layout.Component {
	if state.FullscreenTab == FullscreenTabEditor {
		div := newEditorDiv(
			state.Editor,
			layout.Unit{Type: layout.UnitPercent, Value: 100},
			layout.Unit{Type: layout.UnitPercent, Value: 100},
		)
		div.SetPadding(layout.Padding{
			Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
			Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
			Top:    layout.Unit{Type: layout.UnitPx, Value: 3},
			Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
		})
		return div
	}

	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	outputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 3},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	if state.FullscreenOutputView() == InfoViewGitDiff {
		outputDiv.AppendChild(newGitDiffLog(state))
	} else {
		outputDiv.AppendChild(newChatLog(state))
	}
	return outputDiv
}

func newHorizontalRule(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
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
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	return div
}

func newSplitStatusBar(state *State) *components.Div {
	div := components.NewDiv()
	div.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	style.SetForeground(100, 98, 112)
	div.SetStyle(style)

	bar := components.NewStatusBar()
	bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	bar.SetSession(state.SessionTitle)
	bar.SetMode(state.Mode)
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

func chatStatusText(state *State) string {
	icon := '•'
	label := "offline"
	switch {
	case state.ServerStarting:
		icon = '◌'
		label = "starting"
	case state.IsStreaming():
		icon = '◌'
		label = "thinking"
	case state.ServerReady:
		label = "ready"
	}
	model := "model pending"
	if state.ModelProvider != "" && state.ModelID != "" {
		model = state.ModelProvider + "/" + state.ModelID
	} else if state.ModelID != "" {
		model = state.ModelID
	}
	mode := state.Mode
	if mode == "" {
		mode = "build"
	}
	return string(icon) + " " + label + "  " + mode + "  " + model
}

func newInputBoxDiv(ed *components.Editor, w, h layout.Unit, status string) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(22, 20, 27)
	style.SetForeground(220, 228, 216)
	div.SetStyle(style)
	box := components.NewInputBox(ed)
	box.SetStatus(status)
	div.AppendChild(box)
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
	popup.SetQuery(state.PaletteSearch())
	popup.SetItems(state.PaletteItems(), state.PaletteIndex)
	div.AppendChild(popup)
	return div
}

// ── Layout builders ───────────────────────────────────────────────────────────

func CreateDefaultLayout(state *State) *components.Div {
	return CreateChatLayout(state)
}

func CreateChatLayout(state *State) *components.Div {
	return CreateSessionsLayout(state)
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
			layout.Unit{Type: layout.UnitPercent, Value: float64(state.SplitLeftPercent())},
			layout.Unit{Type: layout.UnitPercent, Value: 100},
		)
	} else {
		leftDiv = newEditorDiv(
			state.Editor,
			layout.Unit{Type: layout.UnitPercent, Value: float64(state.SplitLeftPercent())},
			layout.Unit{Type: layout.UnitPercent, Value: 100},
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
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	rightDiv.AppendChild(newInfoTabs(state))
	rightDiv.AppendChild(newInfoView(state))
	rightDiv.AppendChild(newSplitStatusBar(state))

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	root.SetDirection(layout.Row)
	root.AppendChild(leftDiv)
	root.AppendChild(sep)
	root.AppendChild(rightDiv)
	return root
}

func CreateSessionsLayout(state *State) *components.Div {
	if state.width < MinSplitLayoutWidth {
		return CreateNarrowSessionsLayout(state)
	}

	leftDiv := components.NewDiv()
	leftDiv.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 15},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	leftDiv.AppendChild(newSessions(state, components.SessionsVertical))

	sep := newVerticalSeparator()
	statusSep := components.NewSeparator()
	sep.AppendChild(statusSep)

	rightDiv := newOutput()
	rightDiv.SetSize(
		layout.Unit{Type: layout.UnitGrow, Value: 1},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	rightDiv.SetPadding(layout.Padding{})

	chatDiv := newOutput()
	chatDiv.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	chatDiv.AppendChild(newChatLog(state))

	inputDiv := newInputBoxDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 9},
		chatStatusText(state),
	)
	inputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 0},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	rightDiv.AlignItems(layout.ACenter)
	rightDiv.AppendChild(chatDiv)
	rightDiv.AppendChild(inputDiv)

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
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
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 50},
	)
	outputDiv.AppendChild(newChatLog(state))

	sepDiv := newHorizontalRule(state)

	inputDiv := newEditorDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPercent, Value: 100},
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
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	root.AppendChild(outputDiv)
	root.AppendChild(sepDiv)
	root.AppendChild(inputDiv)
	return root
}

func CreateNarrowSessionsLayout(state *State) *components.Div {
	sessionsDiv := components.NewDiv()
	sessionsDiv.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 3},
	)
	sessionsDiv.AppendChild(newSessions(state, components.SessionsHorizontal))

	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	outputDiv.AppendChild(newChatLog(state))

	inputDiv := newInputBoxDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 9},
		chatStatusText(state),
	)
	inputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 0},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	root.AlignItems(layout.ACenter)
	root.AppendChild(sessionsDiv)
	root.AppendChild(outputDiv)
	root.AppendChild(inputDiv)
	return root
}

func CreateFullscreenLayout(state *State) *components.Div {
	if div, ok := newFullscreenView(state).(*components.Div); ok {
		return div
	}
	return CreateDefaultLayout(state)
}
