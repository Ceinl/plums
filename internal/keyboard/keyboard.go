package keyboard

import (
	"context"
	"os"
	"unicode/utf8"
)

type Event struct {
	Type EventType
	Ch   rune
	Raw  []byte
}

const (
	byteEnter      = 13
	byteBackspace  = 127
	byteCtrlC      = 3
	byteArrowUp    = 65
	byteArrowDown  = 66
	byteArrowRight = 67
	byteArrowLeft  = 68
	byteEscape     = 27
	byteBracket    = 91
)

type EventType int

const (
	KeyRune EventType = iota
	KeyEnter
	KeyBackspace
	KeyCtrlC
	KeyArrowUp
	KeyArrowDown
	KeyArrowRight
	KeyArrowLeft
	KeyUnknown
)

func Listen(ctx context.Context) <-chan Event {
	out := make(chan Event)
	go func() {
		defer close(out)
		buf := make([]byte, 3)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				continue
			}
			ev := parseKeys(buf[:n])
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}

	}()
	return out
}

func parseKeys(b []byte) Event {
	if len(b) == 0 {
		return Event{Type: KeyUnknown, Raw: b}
	}

	switch {
	// One byte keys
	case b[0] == byteEnter:
		return Event{Type: KeyEnter, Raw: b}
	case b[0] == byteBackspace:
		return Event{Type: KeyBackspace, Raw: b}
	case b[0] == byteCtrlC:
		return Event{Type: KeyCtrlC, Raw: b}

	// Escape sequence keys
	case len(b) == 3 && b[0] == byteEscape && b[1] == byteBracket && b[2] == byteArrowUp:
		return Event{Type: KeyArrowUp, Raw: b}
	case len(b) == 3 && b[0] == byteEscape && b[1] == byteBracket && b[2] == byteArrowDown:
		return Event{Type: KeyArrowDown, Raw: b}
	case len(b) == 3 && b[0] == byteEscape && b[1] == byteBracket && b[2] == byteArrowRight:
		return Event{Type: KeyArrowRight, Raw: b}
	case len(b) == 3 && b[0] == byteEscape && b[1] == byteBracket && b[2] == byteArrowLeft:
		return Event{Type: KeyArrowLeft, Raw: b}

	// Default
	default:
		r, _ := utf8.DecodeRune(b)
		return Event{Type: KeyRune, Ch: r, Raw: b}
	}
}
