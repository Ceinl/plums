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
			switch ev.Type {
			case keyboard.KeyCtrlC:
				return
			case keyboard.KeyEnter:
				state.SubmitInput()
				app.Render(state)
				continue
			case keyboard.KeyBackspace:
				state.PopInput()
				app.Render(state)
				continue
			case keyboard.KeyRune:
				state.AppendInput(ev.Ch)
				app.Render(state)
				continue
			}

		case s := <-stream:
			state.AppendAiOutput(s)
			app.Render(state)
		}
	}
}
