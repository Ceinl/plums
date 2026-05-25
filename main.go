package main

import (
	"context"
	"flag"
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
	session *ai.Session
	server  *ai.ServerProcess
	err     error
}

func main() {
	defer debuglog.Close()

	configPath := flag.String("config", "", "path to plums config file")
	flag.Parse()
	if *configPath != "" {
		debuglog.Printf("config: using %s", *configPath)
	} else {
		debuglog.Printf("config: no config file specified")
	}

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
				refreshSessionModel(ctx, state, client)
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
				applySession(state, result.session)
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
	out <- startupResult{session: session, server: serverProc}
}

func applySession(state *app.State, session *ai.Session) {
	if session == nil {
		return
	}
	state.SetSessionID(session.ID)
	if session.Model != nil {
		state.SetModel(session.Model.ProviderID, session.Model.ID)
	}
}

func refreshSessionModel(ctx context.Context, state *app.State, client *ai.Client) {
	if state.SessionID == "" {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	session, err := client.GetSession(refreshCtx, state.SessionID)
	if err != nil {
		debuglog.Printf("session: refresh model failed: %v", err)
		return
	}
	applySession(state, session)
}

func handlePaletteAction(ctx context.Context, state *app.State, client *ai.Client, action app.PaletteAction) {
	switch action {
	case app.PaletteActionNewSession:
		session, err := client.CreateSession(ctx)
		if err != nil {
			state.AddMessage("system", fmt.Sprintf("failed to create session: %v", err))
			return
		}
		applySession(state, session)
		if session.Model == nil {
			state.SetModel("", "")
		}
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
