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

// ── Layout builders ───────────────────────────────────────────────────────────

func CreateDefaultLayout(state *State) *components.Div {
	return CreateZenLayout(state)
}

// CreateZenLayout is the Go fallback for the minimalistic "zen" layout: a
// single neutral-grey column with the chat output above a roomy input box and
// no sidebar, separators, or tabs.
func CreateZenLayout(state *State) *components.Div {
	chatDiv := components.NewDiv()
	chatDiv.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	chatDiv.SetStyle(themedBg(theme.ZenBg))
	chatDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 6},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 6},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 2},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 1},
	})
	chatDiv.AppendChild(newChatLog(state))

	inputDiv := newInputBoxDiv(
		state.Editor,
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 7},
		chatStatusSegments(state),
	)
	inputDiv.SetStyle(themedStyle(theme.ZenBg, theme.ZenText))
	inputDiv.SetPadding(layout.Padding{
		Left:   layout.Unit{Type: layout.UnitPx, Value: 6},
		Right:  layout.Unit{Type: layout.UnitPx, Value: 6},
		Top:    layout.Unit{Type: layout.UnitPx, Value: 0},
		Bottom: layout.Unit{Type: layout.UnitPx, Value: 2},
	})

	root := components.NewDiv()
	root.SetSize(
		layout.Unit{Type: layout.UnitPercent, Value: 100},
		layout.Unit{Type: layout.UnitPercent, Value: 100},
	)
	root.SetStyle(themedBg(theme.ZenBg))
	root.AlignItems(layout.ACenter)
	root.AppendChild(chatDiv)
	root.AppendChild(inputDiv)
	return root
}
