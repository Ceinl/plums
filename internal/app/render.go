package app

import (
	"strings"

	"plums/internal/components"
	"plums/internal/layout"
	"plums/internal/screen"
)

var scr *screen.Screen

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

	outputDiv := components.NewDiv()
	outputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitGrow, Value: 1},
	)
	outputText := components.NewText()
	outputText.SetContent(state.aioutput)
	outputDiv.AppendChild(outputText)
	outputDiv.SetPadding(layout.Padding{Left: layout.Unit{Type: layout.UnitPx, Value: 3}, Right: layout.Unit{Type: layout.UnitPx, Value: 3}, Top: layout.Unit{Type: layout.UnitPx, Value: 1}, Bottom: layout.Unit{Type: layout.UnitPx, Value: 1}})

	sepDiv := components.NewDiv()
	sepDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	sepText := components.NewText()
	sepText.SetContent(strings.Repeat("─", state.width))
	sepDiv.AppendChild(sepText)

	inputDiv := components.NewDiv()
	inputDiv.SetSize(
		layout.Unit{Type: layout.UnitPersent, Value: 100},
		layout.Unit{Type: layout.UnitPx, Value: 1},
	)
	inputText := components.NewText()
	inputText.SetContent("> " + state.input)
	inputDiv.AppendChild(inputText)

	root.AppendChild(outputDiv)
	root.AppendChild(sepDiv)
	root.AppendChild(inputDiv)

	root.Layout(0, 0, state.width, state.height)
	root.Render(scr)

	scr.Flush()
	scr.SetCursor(2+len([]rune(state.input)), state.height-1)
	scr.ShowCursor()
}
