package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"plums/internal/ai"
	"plums/internal/app"
	"plums/internal/debuglog"
	"plums/internal/keyboard"
	"plums/internal/ui"
)

type startupResult struct {
	sessionID string
	server    *ai.ServerProcess
	err       error
}

func main() {
	defer debuglog.Close()

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
	var serverProc *ai.ServerProcess
	defer func() { serverProc.Stop() }()

	startupCh := make(chan startupResult, 1)
	state.SetServerStarting(true)
	go startOpencode(ctx, client, startupCh)

	app.Render(state)

	// Spinner ticker: re-render at 80 ms so the Braille animation is smooth
	// even while the model is thinking (no tokens arriving yet).
	spinTicker := time.NewTicker(80 * time.Millisecond)
	defer spinTicker.Stop()

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
			if action := state.ConsumePendingAction(); action != app.PaletteActionNone {
				handlePaletteAction(ctx, state, client, action)
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
						} else if last.Role == "user" {
							state.AddMessage("system", "no active session – is opencode serve running?")
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
		case <-spinTicker.C:
			if state.IsStreaming() {
				app.Render(state)
			}
		case result := <-startupCh:
			state.SetServerStarting(false)
			if result.err != nil {
				state.AddMessage("system", result.err.Error())
			} else {
				serverProc = result.server
				state.SetServerReady(true)
				state.SessionID = result.sessionID
			}
			app.Render(state)
		case <-sigCh:
			t.RefreshSize()
			state.Resize(t.W, t.H)
			app.Render(state)
		}
	}
}

func startOpencode(ctx context.Context, client *ai.Client, out chan<- startupResult) {
	var serverProc *ai.ServerProcess
	debuglog.Printf("startup: checking opencode health")
	if err := client.Health(ctx); err != nil {
		debuglog.Printf("startup: health check failed: %v", err)
		proc, err := ai.StartServer(ctx)
		if err != nil {
			debuglog.Printf("startup: start server failed: %v", err)
			out <- startupResult{err: fmt.Errorf("failed to start opencode server: %w", err)}
			return
		}
		serverProc = proc
		debuglog.Printf("startup: started opencode server process")
		if err := ai.WaitForHealth(ctx, client, 10*time.Second); err != nil {
			debuglog.Printf("startup: wait for health failed: %v", err)
			serverProc.Stop()
			out <- startupResult{err: fmt.Errorf("failed to start opencode server: %w", err)}
			return
		}
	} else {
		debuglog.Printf("startup: existing opencode server is healthy")
	}

	debuglog.Printf("startup: creating session")
	session, err := client.CreateSession(ctx)
	if err != nil {
		debuglog.Printf("startup: create session failed: %v", err)
		serverProc.Stop()
		out <- startupResult{err: fmt.Errorf("failed to create session: %w", err)}
		return
	}
	debuglog.Printf("startup: session ready: %s", session.ID)
	out <- startupResult{sessionID: session.ID, server: serverProc}
}

func handleKey(state *app.State, ev keyboard.Event) (handled bool, quit bool) {
	ed := state.Editor
	if state.PopupOpen {
		switch ev.Type {
		case keyboard.KeyEscape:
			state.ClosePalette()
			return true, false
		case keyboard.KeyArrowUp:
			state.MovePalette(-1)
			return true, false
		case keyboard.KeyArrowDown:
			state.MovePalette(1)
			return true, false
		case keyboard.KeyEnter:
			state.SelectPaletteItem()
			return true, false
		case keyboard.KeyRune:
			if ev.Ctrl && (ev.Ch == 'P' || ev.Ch == 'p') {
				state.ClosePalette()
				return true, false
			}
		}
		return true, false
	}

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
		if state.PopupOpen {
			state.TogglePopup()
			return true, false
		}
		ed.ClearSelection()
		return true, false
	case keyboard.KeyRune:
		if ev.Ctrl && (ev.Ch == 'P' || ev.Ch == 'p') {
			state.TogglePopup()
			return true, false
		}
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
				ed.MoveCursorHome()
				return true, false
			case 'e', 'E':
				ed.MoveCursorEnd()
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
		if ev.Alt && !ev.Shift && !ev.Ctrl {
			state.ScrollOutput(1)
			return true, false
		}
		if ev.Shift {
			ed.SelectUp()
		} else {
			ed.MoveCursorUp()
		}
		return true, false
	case keyboard.KeyArrowDown:
		if ev.Alt && !ev.Shift && !ev.Ctrl {
			state.ScrollOutput(-1)
			return true, false
		}
		if ev.Shift {
			ed.SelectDown()
		} else {
			ed.MoveCursorDown()
		}
		return true, false
	case keyboard.KeyPageUp:
		state.ScrollOutputPage(1)
		return true, false
	case keyboard.KeyPageDown:
		state.ScrollOutputPage(-1)
		return true, false
	case keyboard.KeyMouseWheelUp:
		state.ScrollOutput(3)
		return true, false
	case keyboard.KeyMouseWheelDown:
		state.ScrollOutput(-3)
		return true, false
	case keyboard.KeyHome:
		if ev.Ctrl {
			state.ScrollOutputPage(1 << 20)
			return true, false
		}
		if ev.Shift {
			ed.SelectHome()
		} else {
			ed.MoveCursorHome()
		}
		return true, false
	case keyboard.KeyEnd:
		if ev.Ctrl {
			state.ScrollOutputBottom()
			return true, false
		}
		if ev.Shift {
			ed.SelectEnd()
		} else {
			ed.MoveCursorEnd()
		}
		return true, false
	}
	return false, false
}

func handlePaletteAction(ctx context.Context, state *app.State, client *ai.Client, action app.PaletteAction) {
	switch action {
	case app.PaletteActionNewSession:
		session, err := client.CreateSession(ctx)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to create session: %v", err))
			return
		}
		state.SetSessionID(session.ID)
		state.ClearConversation()
		state.AddMessage("system", "started new session "+session.ID)
	case app.PaletteActionSwitchMode:
		state.ToggleMode()
		state.AddMessage("system", "switched to "+state.Mode+" mode")
	case app.PaletteActionChangeModel:
		state.AddMessage("system", "model switching needs opencode model API wiring")
	case app.PaletteActionSessionsList:
		state.AddMessage("system", "sessions list will be added after session listing API is wired")
	}
}
