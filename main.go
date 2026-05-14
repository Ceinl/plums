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

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	state := app.NewState(t.W, t.H)

	client := ai.NewClient()
	if err := client.Health(ctx); err == nil {
		session, err := client.CreateSession(ctx)
		if err == nil {
			state.SessionID = session.ID
		}
	}

	app.Render(state)

	var aiStream <-chan string
	var cancelStream context.CancelFunc

	for {
		select {
		case ev := <-keys:
			handled, quit := handleKey(state, ev)
			if quit {
				if cancelStream != nil {
					cancelStream()
				}
				cancel()
				return
			}
			if handled {
				if ev.Type == keyboard.KeyEnter && ev.Alt {
					msgs := state.Messages()
					if len(msgs) > 0 {
						last := msgs[len(msgs)-1]
						if last.Role == "user" && state.SessionID != "" {
							if cancelStream != nil {
								cancelStream()
							}
							var sctx context.Context
							sctx, cancelStream = context.WithCancel(ctx)
							state.SetStreaming(true)
							state.ClearAiOutput()
							aiStream = client.SendMessage(sctx, state.SessionID, last.Content)
						}
					}
				}
				app.Render(state)
			}
		case s, ok := <-aiStream:
			if ok {
				state.AppendAiOutput(s)
				app.Render(state)
			} else {
				state.FinalizeAiOutput()
				aiStream = nil
				cancelStream = nil
				app.Render(state)
			}
		case <-sigCh:
			t.RefreshSize()
			state.Resize(t.W, t.H)
			app.Render(state)
		}
	}
}

func handleKey(state *app.State, ev keyboard.Event) (handled bool, quit bool) {
	ed := state.Editor

	switch ev.Type {
	case keyboard.KeyCtrlC:
		return false, true
	case keyboard.KeyEnter:
		if !ev.Alt {
			ed.InsertNewline()
			return true, false
		}
		state.SubmitInput()
		return true, false
	case keyboard.KeyBackspace:
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordBackward()
		} else {
			ed.DeleteBackward()
		}
		return true, false
	case keyboard.KeyDelete:
		if ev.Alt || ev.Ctrl {
			ed.DeleteWordForward()
		} else {
			ed.DeleteForward()
		}
		return true, false
	case keyboard.KeyTab:
		state.SwitchLayout()
		return true, false
	case keyboard.KeyEscape:
		ed.ClearSelection()
		return true, false
	case keyboard.KeyRune:
		// Alt+b / Alt+f: readline-style word jump (emitted by Terminal.app and
		// many other macOS terminals when the user presses Option+Left/Right).
		if ev.Alt && !ev.Ctrl {
			switch ev.Ch {
			case 'b', 'B': // Option+Left in Terminal.app
				ed.MoveWordLeft()
				return true, false
			case 'f', 'F': // Option+Right in Terminal.app
				ed.MoveWordRight()
				return true, false
			case 'd', 'D': // Alt+d = delete word forward (readline)
				ed.DeleteWordForward()
				return true, false
			}
		}
		if ev.Ctrl {
			switch ev.Ch {
			case 'a', 'A':
				ed.SelectAll()
				return true, false
			case 'k', 'K':
				ed.DeleteCurrentLine()
				return true, false
			case 's', 'S':
				state.SubmitInput()
				return true, false
			}
			return false, false
		}
		if ev.Alt {
			// Don't insert Alt-modified characters into the editor.
			return false, false
		}
		ed.InsertRune(ev.Ch)
		return true, false
	case keyboard.KeyArrowLeft:
		switch {
		case (ev.Ctrl || ev.Alt) && ev.Shift:
			ed.SelectWordLeft()
		case ev.Ctrl || ev.Alt:
			ed.MoveWordLeft()
		case ev.Shift:
			ed.SelectLeft()
		default:
			ed.MoveCursorLeft()
		}
		return true, false
	case keyboard.KeyArrowRight:
		switch {
		case (ev.Ctrl || ev.Alt) && ev.Shift:
			ed.SelectWordRight()
		case ev.Ctrl || ev.Alt:
			ed.MoveWordRight()
		case ev.Shift:
			ed.SelectRight()
		default:
			ed.MoveCursorRight()
		}
		return true, false
	case keyboard.KeyArrowUp:
		if ev.Shift {
			ed.SelectUp()
		} else {
			ed.MoveCursorUp()
		}
		return true, false
	case keyboard.KeyArrowDown:
		if ev.Shift {
			ed.SelectDown()
		} else {
			ed.MoveCursorDown()
		}
		return true, false
	case keyboard.KeyHome:
		if ev.Shift {
			ed.SelectHome()
		} else {
			ed.MoveCursorHome()
		}
		return true, false
	case keyboard.KeyEnd:
		if ev.Shift {
			ed.SelectEnd()
		} else {
			ed.MoveCursorEnd()
		}
		return true, false
	}
	return false, false
}
