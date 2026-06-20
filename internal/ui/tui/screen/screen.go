package screen

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const DefaultBg = "\x1b[48;2;25;23;29m"
const DefaultFg = "\x1b[38;2;200;200;200m"
const DefaultDecor = ""

type Cell struct {
	Ch    rune
	Bg    string
	Fg    string
	Decor string
}

type Screen struct {
	w, h int
	old  [][]Cell
	cur  [][]Cell
	out  io.Writer
}

func NewScreen(w, h int) *Screen {
	s := &Screen{
		w:   w,
		h:   h,
		out: os.Stdout,
	}
	s.resize(w, h)
	return s
}

func (s *Screen) resize(w, h int) {
	s.w, s.h = w, h
	s.old = make([][]Cell, h)
	s.cur = make([][]Cell, h)
	for i := 0; i < h; i++ {
		s.old[i] = make([]Cell, w)
		s.cur[i] = make([]Cell, w)
		for j := 0; j < w; j++ {
			s.old[i][j] = Cell{}
			s.cur[i][j] = Cell{Ch: ' ', Bg: DefaultBg, Fg: DefaultFg}
		}
	}
}

func (s *Screen) Width() int  { return s.w }
func (s *Screen) Height() int { return s.h }

// SetOutput redirects flushed terminal bytes. It is primarily useful for tests
// that need to assert rendered output without writing to stdout.
func (s *Screen) SetOutput(out io.Writer) {
	if out == nil {
		out = os.Stdout
	}
	s.out = out
}

// Cell returns the current cell at (x, y); a zero Cell for out-of-range coords.
// Intended for tests and diagnostics.
func (s *Screen) Cell(x, y int) Cell {
	if x < 0 || x >= s.w || y < 0 || y >= s.h {
		return Cell{}
	}
	return s.cur[y][x]
}

func (s *Screen) Clear() {
	for i := 0; i < s.h; i++ {
		for j := 0; j < s.w; j++ {
			s.cur[i][j] = Cell{Ch: ' ', Bg: DefaultBg, Fg: DefaultFg}
		}
	}
}

func (s *Screen) Set(x, y int, ch rune, fg, bg, decor string) {
	if fg == "" {
		fg = DefaultFg
	}
	if bg == "" {
		bg = DefaultBg
	}
	if decor == "" {
		decor = DefaultDecor
	}
	if x < 0 || x >= s.w || y < 0 || y >= s.h {
		return
	}
	s.cur[y][x] = Cell{Ch: ch, Bg: bg, Fg: fg, Decor: decor}
}

func (s *Screen) Flush() {
	fmt.Fprint(s.out, "\x1b[?25l")
	for y := 0; y < s.h; y++ {
		changed := false
		startX := -1
		for x := 0; x < s.w; x++ {
			if s.cur[y][x] != s.old[y][x] {
				if !changed {
					changed = true
					startX = x
				}
			} else if changed {
				s.flushSegment(y, startX, x)
				changed = false
			}
		}
		if changed {
			s.flushSegment(y, startX, s.w)
		}
		copy(s.old[y], s.cur[y])
	}
	fmt.Fprint(s.out, "\x1b[0m")
}

func (s *Screen) flushSegment(y, startX, endX int) {
	fmt.Fprintf(s.out, "\x1b[%d;%dH", y+1, startX+1)

	var (
		b           strings.Builder
		activeFg    string
		activeBg    string
		activeDecor string
	)

	for x := startX; x < endX; x++ {
		cell := s.cur[y][x]

		if cell.Fg != activeFg || cell.Bg != activeBg || cell.Decor != activeDecor {
			if b.Len() > 0 {
				fmt.Fprint(s.out, b.String())
				b.Reset()
			}
			fmt.Fprint(s.out, "\x1b[0m")
			if cell.Decor != "" {
				fmt.Fprint(s.out, cell.Decor)
			}
			fmt.Fprint(s.out, cell.Bg, cell.Fg)

			activeFg = cell.Fg
			activeBg = cell.Bg
			activeDecor = cell.Decor
		}
		b.WriteRune(cell.Ch)
	}
	if b.Len() > 0 {
		fmt.Fprint(s.out, b.String())
	}
}

func (s *Screen) SetCursor(x, y int) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= s.w {
		x = s.w - 1
	}
	if y >= s.h {
		y = s.h - 1
	}
	fmt.Fprintf(s.out, "\x1b[%d;%dH", y+1, x+1)
}

func (s *Screen) ShowCursor() {
	fmt.Fprint(s.out, "\x1b[?25h")
}
