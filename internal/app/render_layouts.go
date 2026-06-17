package app

import (
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
	"github.com/Ceinl/plums/internal/ui/tui/theme"
)

// ── Style helpers ─────────────────────────────────────────────────────────────
//
// All colours come from the theme package so every layout shares one palette:
//   theme.BgBase  – chat / output areas
//   theme.BgPanel – editor and palette panes

func themedBg(bg theme.Color) layout.Style {
	style := layout.Style{}
	style.SetBackground(bg.R, bg.G, bg.B)
	return style
}

func themedStyle(bg, fg theme.Color) layout.Style {
	style := themedBg(bg)
	style.SetForeground(fg.R, fg.G, fg.B)
	return style
}

// ── Factory helpers ───────────────────────────────────────────────────────────

func newOutput() *components.Div {
	outputDiv := components.NewDiv()
	outputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 2},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 2},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	outputDiv.SetStyle(themedBg(theme.BgBase))
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
	chatLog.SetToolCallVisibility(state.ToolCallMode)
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
	div.SetStyle(themedBg(theme.BgBase))

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
	div.SetStyle(themedStyle(theme.BgBase, theme.TextFaint))

	bar := components.NewStatusBar()
	bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	bar.SetSession(state.SessionTitle)
	bar.SetMode(state.Mode)
	bar.SetModel(state.ModelProvider, state.ModelID)
	div.AppendChild(bar)
	return div
}

func newEditorDiv(ed *components.Editor, w, h layout.Unit) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)
	div.SetStyle(themedStyle(theme.BgPanel, theme.Text))
	div.AppendChild(ed)
	return div
}

// chatStatusSegments builds the coloured status line above the input box: the
// state indicator in its status colour, the mode in accent, the model muted.
func chatStatusSegments(state *State) []components.StatusSegment {
	icon, label, fg := "•", "offline", theme.TextFaint
	switch {
	case state.ServerStarting:
		icon, label, fg = "◌", "starting", theme.StatusStarting
	case state.IsStreaming():
		icon, label, fg = "◌", "thinking", theme.StatusThinking
	case state.ServerReady:
		label, fg = "ready", theme.StatusReady
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
	return []components.StatusSegment{
		{Text: icon + " " + label, Fg: fg.Fg()},
		{Text: "  " + mode, Fg: theme.Accent.Fg()},
		{Text: "  " + model, Fg: theme.TextFaint.Fg()},
	}
}

func newInputBoxDiv(ed *components.Editor, w, h layout.Unit, status []components.StatusSegment) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	div.SetStyle(themedStyle(theme.BgBase, theme.Text))
	box := components.NewInputBox(ed)
	box.SetStatusSegments(status)
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

	div.SetStyle(themedStyle(theme.BgPanel, theme.Text))

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
		chatStatusSegments(state),
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
		chatStatusSegments(state),
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
