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
				if ev.Type == keyboard.KeyEnter && !ev.Alt {
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
