package keyboard

import (
	"context"
	"os"
	"time"
	"unicode/utf8"
)

type Event struct {
	Type  EventType
	Ch    rune
	Shift bool
	Ctrl  bool
	Alt   bool
	Raw   []byte
}

const (
	byteEnter      = 13
	byteBackspace  = 127
	byteCtrlC      = 3
	byteTab        = 9
	byteEscape     = 27
	byteBracket    = 91
	byteO          = 79
	byteTilde      = 126
	byteSemicolon  = 59
	byteNum2       = 50
	byteNum5       = 53
)

type EventType int

const (
	KeyRune EventType = iota
	KeyEnter
	KeyBackspace
	KeyCtrlC
	KeyTab
	KeyEscape
	KeyArrowUp
	KeyArrowDown
	KeyArrowRight
	KeyArrowLeft
	KeyHome
	KeyEnd
	KeyDelete
	KeyUnknown
)

func Listen(ctx context.Context) <-chan Event {
	out := make(chan Event)
	go func() {
		defer close(out)
		buf := make([]byte, 1)
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
			b := buf[0]

			if b == byteEscape {
				ev := readEscapeSequence(buf)
				select {
				case <-ctx.Done():
					return
				case out <- ev:
				}
				continue
			}

			ev := parseSingleByte(b)
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out
}

func readByteTimeout(timeout time.Duration) (byte, bool) {
	ch := make(chan byte, 1)
	go func() {
		buf := make([]byte, 1)
		n, _ := os.Stdin.Read(buf)
		if n > 0 {
			ch <- buf[0]
		}
	}()
	select {
	case b := <-ch:
		return b, true
	case <-time.After(timeout):
		return 0, false
	}
}

func readEscapeSequence(buf []byte) Event {
	// We already read ESC. Read the next byte with timeout.
	next, ok := readByteTimeout(50 * time.Millisecond)
	if !ok {
		return Event{Type: KeyEscape}
	}

	if next == byteBracket {
		// CSI sequence: ESC [ ... final_byte
		return readCSI()
	}

	if next == byteO {
		// SS3 sequence: ESC O X
		next2, ok2 := readByteTimeout(50 * time.Millisecond)
		if !ok2 {
			return Event{Type: KeyUnknown}
		}
		return parseSS3(next2)
	}

	// ESC followed by a regular key = Alt+key
	if next < 0x80 {
		if next == byteEnter {
			return Event{Type: KeyEnter, Alt: true}
		}
		ev := parseSingleByte(next)
		ev.Alt = true
		return ev
	}

	return Event{Type: KeyUnknown}
}

func readCSI() Event {
	// Read parameters and final byte
	params := make([]byte, 0, 8)
	final := byte(0)

	for {
		b, ok := readByteTimeout(50 * time.Millisecond)
		if !ok {
			break
		}
		if b >= 0x40 && b <= 0x7E {
			final = b
			break
		}
		params = append(params, b)
	}

	return parseCSI(params, final)
}

func parseCSI(params []byte, final byte) Event {
	var paramNums []int
	var current int
	for _, b := range params {
		if b == byteSemicolon {
			paramNums = append(paramNums, current)
			current = 0
			continue
		}
		if b >= '0' && b <= '9' {
			current = current*10 + int(b-'0')
		}
	}
	paramNums = append(paramNums, current)

	modifier := 1 // default: no modifier
	if len(paramNums) >= 2 {
		modifier = paramNums[len(paramNums)-1]
	}
	if len(paramNums) == 0 {
		paramNums = append(paramNums, 1)
	}

	shift := modifier == 2 || modifier == 4 || modifier == 6 || modifier == 8
	ctrl := modifier == 5 || modifier == 6 || modifier == 7 || modifier == 8
	alt := modifier == 3 || modifier == 4 || modifier == 7 || modifier == 8

	switch final {
	case 'A':
		return Event{Type: KeyArrowUp, Shift: shift, Ctrl: ctrl, Alt: alt}
	case 'B':
		return Event{Type: KeyArrowDown, Shift: shift, Ctrl: ctrl, Alt: alt}
	case 'C':
		return Event{Type: KeyArrowRight, Shift: shift, Ctrl: ctrl, Alt: alt}
	case 'D':
		return Event{Type: KeyArrowLeft, Shift: shift, Ctrl: ctrl, Alt: alt}
	case 'H':
		return Event{Type: KeyHome, Shift: shift, Ctrl: ctrl, Alt: alt}
	case 'F':
		return Event{Type: KeyEnd, Shift: shift, Ctrl: ctrl, Alt: alt}
	case '~':
		code := paramNums[0]
		switch code {
		case 1:
			return Event{Type: KeyHome, Shift: shift, Ctrl: ctrl, Alt: alt}
		case 3:
			return Event{Type: KeyDelete, Shift: shift, Ctrl: ctrl, Alt: alt}
		case 4:
			return Event{Type: KeyEnd, Shift: shift, Ctrl: ctrl, Alt: alt}
		}
	}

	return Event{Type: KeyUnknown}
}

func parseSS3(b byte) Event {
	switch b {
	case 'A':
		return Event{Type: KeyArrowUp}
	case 'B':
		return Event{Type: KeyArrowDown}
	case 'C':
		return Event{Type: KeyArrowRight}
	case 'D':
		return Event{Type: KeyArrowLeft}
	case 'H':
		return Event{Type: KeyHome}
	case 'F':
		return Event{Type: KeyEnd}
	}
	return Event{Type: KeyUnknown}
}

func parseSingleByte(b byte) Event {
	switch b {
	case byteEnter:
		return Event{Type: KeyEnter}
	case byteBackspace:
		return Event{Type: KeyBackspace}
	case byteCtrlC:
		return Event{Type: KeyCtrlC}
	case byteTab:
		return Event{Type: KeyTab}
	case 0:
		return Event{Type: KeyCtrlC}
	}

	if b < 0x20 {
		// Control character
		return Event{Type: KeyRune, Ch: rune(b + 0x40), Ctrl: true}
	}

	r, _ := utf8.DecodeRune([]byte{b})
	return Event{Type: KeyRune, Ch: r}
}
