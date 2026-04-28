package ui

import (
	"fmt"

	"golang.org/x/term"
)

const (
	OPEN_ALT     = "\x1b[?1049h"
	CLOSE_ALT    = "\x1b[?1049l"
	HIDE_CURSOR  = "\x1b[?25l"
	SHOW_CURSOR  = "\x1b[?25h"
	CLEAR_SCREEN = "\x1b[2J"
	MOVE_CURSOR  = "\x1b[H"
)

type Terminal struct {
	oldstate *term.State
	fd       int
	W, H     int
}

func NewTerminal(fd int) *Terminal {
	return &Terminal{
		fd: fd,
	}
}

func (t *Terminal) Enter() {
	oldstate, err := term.MakeRaw(t.fd)
	if err != nil {
		panic(err)
	}
	t.RefreshSize()
	t.oldstate = oldstate
	fmt.Print(HIDE_CURSOR, OPEN_ALT, MOVE_CURSOR, CLEAR_SCREEN)
}

func (t *Terminal) Exit() {
	if t.oldstate == nil {
		return
	}

	fmt.Print(SHOW_CURSOR + CLOSE_ALT)
	term.Restore(t.fd, t.oldstate)
}

func (t *Terminal) RefreshSize() error {
	w, h, err := term.GetSize(t.fd)
	if err != nil {
		return err
	}
	t.W, t.H = w, h
	return nil
}
