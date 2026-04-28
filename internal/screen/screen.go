package screen

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type Screen struct {
	w, h int
	old  [][]rune
	cur  [][]rune
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
	s.old = make([][]rune, h)
	s.cur = make([][]rune, h)
	for i := 0; i < h; i++ {
		s.old[i] = make([]rune, w)
		s.cur[i] = make([]rune, w)
		for j := 0; j < w; j++ {
			s.old[i][j] = ' '
			s.cur[i][j] = ' '
		}
	}
}

func (s *Screen) Width() int  { return s.w }
func (s *Screen) Height() int { return s.h }

func (s *Screen) Clear() {
	for i := 0; i < s.h; i++ {
		for j := 0; j < s.w; j++ {
			s.cur[i][j] = ' '
		}
	}
}

func (s *Screen) Set(x, y int, ch rune) {
	if x < 0 || x >= s.w || y < 0 || y >= s.h {
		return
	}
	s.cur[y][x] = ch
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
}

func (s *Screen) flushSegment(y, startX, endX int) {
	fmt.Fprintf(s.out, "\x1b[%d;%dH", y+1, startX+1)
	var b strings.Builder
	for x := startX; x < endX; x++ {
		b.WriteRune(s.cur[y][x])
	}
	fmt.Fprint(s.out, b.String())
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
