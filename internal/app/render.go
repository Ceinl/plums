package app

import "fmt"

const (
	OPEN_ALT     = "\x1b[?1049h"
	CLOSE_ALT    = "\x1b[?1049l"
	HIDE_CURSOR  = "\x1b[?25l"
	SHOW_CURSOR  = "\x1b[?25h"
	CLEAR_SCREEN = "\x1b[2J"
	MOVE_CURSOR  = "\x1b[H"
)

func moveCursor(row, col int) {
	fmt.Printf("\x1b[%d;%dH", row, col)
}

func clearLine() {
	fmt.Print("\x1b[2K")
}

func clearBelow() {
	fmt.Print("\x1b[J")
}

func Render(state *State) {
	fmt.Print(HIDE_CURSOR)
	moveCursor(1, 5)
	lines := wrapText(state.aioutput, state.width-5)
	for i, line := range lines {
		moveCursor(i+1, 1)
		clearLine()
		fmt.Print(line)
	}
	clearBelow()

	for i := len(lines); i < state.height-2; i++ {
		fmt.Print("\x1b[K\r\n")
	}
	for i := 0; i < state.width; i++ {
		fmt.Print("_")
	}
	//fmt.Print(SHOW_CURSOR)
}

func wrapText(text string, width int) []string {
	var lines []string
	for len(text) > width {
		lines = append(lines, text[:width])
		text = text[width:]
	}
	if len(text) > 0 {
		lines = append(lines, text)
	}
	return lines
}
