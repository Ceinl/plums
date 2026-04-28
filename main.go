package main

import (
	"context"
	"fmt"
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

	for {
		select {
		case ev := <-keys:
			switch ev.Type {
			case keyboard.KeyCtrlC:
				return
			case keyboard.KeyEnter:
				fmt.Print("\n\n\n")
				continue
			case keyboard.KeyBackspace:
				state.PopInput(ev.Ch)
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
