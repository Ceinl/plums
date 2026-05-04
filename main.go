package main

import (
	"context"
	"os"
	"plums/internal/ai"
	"plums/internal/app"
	"plums/internal/keyboard"
	"plums/internal/ui"
)

func main() {
	t := ui.NewTerminal(int(os.Stdin.Fd()))
	t.Enter()
	defer t.Exit()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys := keyboard.Listen(ctx)
	stream := ai.RepeatFunc(ctx, ai.SudoText)

	state := app.NewState(t.W, t.H)
	app.Render(state)

	for {
		select {
		case ev := <-keys:
			handled := handleKey(state, ev)
			if handled {
				app.Render(state)
			}
		case s := <-stream:
			state.AppendAiOutput(s)
			app.Render(state)
		}
	}
}

func handleKey(state *app.State, ev keyboard.Event) bool {
	tb := state.TextBox

	switch ev.Type {
	case keyboard.KeyCtrlC:
		os.Exit(0)
	case keyboard.KeyEnter:
		if ev.Alt && tb.IsMultiline() {
			tb.InsertNewline()
			return true
		}
		state.SubmitInput()
		return true
	case keyboard.KeyBackspace:
		tb.DeleteBackward()
		return true
	case keyboard.KeyDelete:
		tb.DeleteForward()
		return true
	case keyboard.KeyTab:
		state.SwitchLayout()
		return true
	case keyboard.KeyEscape:
		tb.ClearSelection()
		return true
	case keyboard.KeyRune:
		if ev.Ctrl {
			switch ev.Ch {
			case 'a', 'A':
				tb.SelectAll()
				return true
			case 's', 'S':
				state.SubmitInput()
				return true
			}
			return false
		}
		tb.InsertRune(ev.Ch)
		return true
	case keyboard.KeyArrowLeft:
		if ev.Shift {
			tb.SelectLeft()
		} else {
			tb.MoveCursorLeft()
		}
		return true
	case keyboard.KeyArrowRight:
		if ev.Shift {
			tb.SelectRight()
		} else {
			tb.MoveCursorRight()
		}
		return true
	case keyboard.KeyArrowUp:
		if ev.Shift {
			tb.SelectUp()
		} else {
			tb.MoveCursorUp()
		}
		return true
	case keyboard.KeyArrowDown:
		if ev.Shift {
			tb.SelectDown()
		} else {
			tb.MoveCursorDown()
		}
		return true
	case keyboard.KeyHome:
		if ev.Shift {
			tb.SelectHome()
		} else {
			tb.MoveCursorHome()
		}
		return true
	case keyboard.KeyEnd:
		if ev.Shift {
			tb.SelectEnd()
		} else {
			tb.MoveCursorEnd()
		}
		return true
	}
	return false
}
