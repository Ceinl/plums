package main

import (
	"context"
	"os"
	"os/signal"
	"plums/internal/ai"
	"plums/internal/app"
	"plums/internal/keyboard"
	"plums/internal/ui"
	"syscall"
)

func main() {
	t := ui.NewTerminal(int(os.Stdin.Fd()))
	t.Enter()
	defer t.Exit()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	keys := keyboard.Listen(ctx)
	stream := ai.RepeatFunc(ctx, ai.SudoText)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	state := app.NewState(t.W, t.H)
	app.Render(state)

	for {
		select {
		case ev := <-keys:
			handled, quit := handleKey(state, ev)
			if quit {
				cancel()
				return
			}
			if handled {
				app.Render(state)
			}
		case s := <-stream:
			state.AppendAiOutput(s)
			app.Render(state)
		case <-sigCh:
			t.RefreshSize()
			state.Resize(t.W, t.H)
			app.Render(state)
		}
	}
}

func handleKey(state *app.State, ev keyboard.Event) (handled bool, quit bool) {
	tb := state.TextBox

	switch ev.Type {
	case keyboard.KeyCtrlC:
		return false, true
	case keyboard.KeyEnter:
		if ev.Alt && tb.IsMultiline() {
			tb.InsertNewline()
			return true, false
		}
		state.SubmitInput()
		return true, false
	case keyboard.KeyBackspace:
		tb.DeleteBackward()
		return true, false
	case keyboard.KeyDelete:
		tb.DeleteForward()
		return true, false
	case keyboard.KeyTab:
		state.SwitchLayout()
		return true, false
	case keyboard.KeyEscape:
		tb.ClearSelection()
		return true, false
	case keyboard.KeyRune:
		if ev.Ctrl {
			switch ev.Ch {
			case 'a', 'A':
				tb.SelectAll()
				return true, false
			case 's', 'S':
				state.SubmitInput()
				return true, false
			}
			return false, false
		}
		tb.InsertRune(ev.Ch)
		return true, false
	case keyboard.KeyArrowLeft:
		if ev.Shift {
			tb.SelectLeft()
		} else {
			tb.MoveCursorLeft()
		}
		return true, false
	case keyboard.KeyArrowRight:
		if ev.Shift {
			tb.SelectRight()
		} else {
			tb.MoveCursorRight()
		}
		return true, false
	case keyboard.KeyArrowUp:
		if ev.Shift {
			tb.SelectUp()
		} else {
			tb.MoveCursorUp()
		}
		return true, false
	case keyboard.KeyArrowDown:
		if ev.Shift {
			tb.SelectDown()
		} else {
			tb.MoveCursorDown()
		}
		return true, false
	case keyboard.KeyHome:
		if ev.Shift {
			tb.SelectHome()
		} else {
			tb.MoveCursorHome()
		}
		return true, false
	case keyboard.KeyEnd:
		if ev.Shift {
			tb.SelectEnd()
		} else {
			tb.MoveCursorEnd()
		}
		return true, false
	}
	return false, false
}
