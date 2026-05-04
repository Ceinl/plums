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
			Left:   layout.Unit{Type: layout.UnitPx, Value: 3},
			Right:  layout.Unit{Type: layout.UnitPx, Value: 3},
			Top:    layout.Unit{Type: layout.UnitPx, Value: 1},
			Bottom: layout.Unit{Type: layout.UnitPx, Value: 1}})

	style := layout.Style{}
	style.SetBackground(30, 27, 35)
	style.SetForeground(255, 255, 255)
	style.AddTextDecoration(layout.Italic)
	style.AddTextDecoration(layout.Bold)
	outputDiv.SetStyle(style)
	return outputDiv
}

func newOutputText(state *State) *components.Text {
	outputText := components.NewText()
	outputText.SetContent(state.aioutput)
	return outputText
}

func newSeparatorDiv(state *State) *components.Div {
	return newTextDiv(strings.Repeat("─", state.width),
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
		30, 27, 35,
	)
}

func newTextDiv(content string, w, h layout.Unit, bgR, bgG, bgB uint8) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(bgR, bgG, bgB)
	style.SetForeground(255, 255, 255)
	style.AddTextDecoration(layout.Italic)
	style.AddTextDecoration(layout.Bold)
	div.SetStyle(style)

	text := components.NewText()
	text.SetContent(content)
	div.AppendChild(text)
	return div
}

func newTextBoxDiv(tb *components.TextBox, w, h layout.Unit, bgR, bgG, bgB uint8) *components.Div {
	div := components.NewDiv()
	div.SetSize(w, h)

	style := layout.Style{}
	style.SetBackground(bgR, bgG, bgB)
	style.SetForeground(255, 255, 255)
	style.AddTextDecoration(layout.Italic)
	style.AddTextDecoration(layout.Bold)
	div.SetStyle(style)
	div.AppendChild(tb)
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
	cx, cy := state.TextBox.CursorScreenPos()
	scr.SetCursor(cx, cy)
	scr.ShowCursor()
}

func CreateDefaultLayout(state *State) *components.Div {
	outputDiv := newOutput()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	outputDiv.AppendChild(newOutputText(state))

	sepDiv := newSeparatorDiv(state)

	inputDiv := newTextBoxDiv(state.TextBox,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
		35, 30, 40,
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
	leftDiv := newOutput()
	leftDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 50},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
	)
	leftDiv.AppendChild(newOutputText(state))

	rightDiv := newTextBoxDiv(state.TextBox,
		layout.Unit{Type: layout.UnitPersent, Value: 50},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		35, 30, 40,
	)

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
	return newTextBoxDiv(state.TextBox,
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		35, 30, 40,
	)
}
