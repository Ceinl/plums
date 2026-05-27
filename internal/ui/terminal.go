package ui

import (
	"fmt"

	"golang.org/x/term"
)

const (
	OPEN_ALT      = "\x1b[?1049h"
	CLOSE_ALT     = "\x1b[?1049l"
	HIDE_CURSOR   = "\x1b[?25l"
	SHOW_CURSOR   = "\x1b[?25h"
	ENABLE_MOUSE  = "\x1b[?1000h\x1b[?1006h"
	DISABLE_MOUSE = "\x1b[?1006l\x1b[?1000l"
	// Ask compatible terminals to distinguish modified keys like Shift+Enter.
	ENABLE_KITTY_KEYBOARD     = "\x1b[>1u"
	DISABLE_KITTY_KEYBOARD    = "\x1b[<u"
	ENABLE_MODIFY_OTHER_KEYS  = "\x1b[>4;2m"
	DISABLE_MODIFY_OTHER_KEYS = "\x1b[>4;0m"
	CLEAR_SCREEN              = "\x1b[2J"
	MOVE_CURSOR               = "\x1b[H"
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

func (t *Terminal) Enter() error {
	oldstate, err := term.MakeRaw(t.fd)
	if err != nil {
		return err
	}
	t.RefreshSize()
	t.oldstate = oldstate
	fmt.Print(HIDE_CURSOR, OPEN_ALT, ENABLE_MOUSE, ENABLE_KITTY_KEYBOARD, ENABLE_MODIFY_OTHER_KEYS, MOVE_CURSOR, CLEAR_SCREEN)
	return nil
}

func (t *Terminal) Exit() {
	if t.oldstate == nil {
		return
	}

	fmt.Print(DISABLE_MODIFY_OTHER_KEYS + DISABLE_KITTY_KEYBOARD + DISABLE_MOUSE + SHOW_CURSOR + CLOSE_ALT)
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
