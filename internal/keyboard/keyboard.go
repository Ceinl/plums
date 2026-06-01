package keyboard

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
	"unicode/utf8"
)

type Event struct {
	Type   EventType
	Ch     rune
	Text   string
	Shift  bool
	Ctrl   bool
	Alt    bool
	Cmd    bool
	Mouse  bool
	MouseX int
	MouseY int
	Raw    []byte
}

const (
	byteEnter     = 13
	byteBackspace = 127
	byteCtrlC     = 3
	byteTab       = 9
	byteEscape    = 27
	byteBracket   = 91
	byteO         = 79
	byteTilde     = 126
	byteSemicolon = 59
	byteNum2      = 50
	byteNum5      = 53
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
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyMouseWheelUp
	KeyMouseWheelDown
	KeyMouseLeftDown
	KeyMouseLeftDrag
	KeyMouseLeftUp
	KeyPaste
	KeyUnknown
)

var bracketedPasteEnd = []byte("\x1b[201~")

type byteReader struct {
	in <-chan byte
}

func (r *byteReader) readByte(ctx context.Context) (byte, bool) {
	select {
	case <-ctx.Done():
		return 0, false
	case b, ok := <-r.in:
		return b, ok
	}
}

func (r *byteReader) readByteTimeout(timeout time.Duration) (byte, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case b, ok := <-r.in:
		return b, ok
	case <-timer.C:
		return 0, false
	}
}

func Listen(ctx context.Context) <-chan Event {
	bytes := readInputBytes(ctx, os.Stdin)
	out := make(chan Event)
	go func() {
		defer close(out)
		reader := &byteReader{in: bytes}
		for {
			b, ok := reader.readByte(ctx)
			if !ok {
				return
			}

			ev := parseByte(reader, b)
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out
}

func readInputBytes(ctx context.Context, input io.Reader) <-chan byte {
	out := make(chan byte, 64)
	go func() {
		defer close(out)
		buf := make([]byte, 1)
		for {
			n, err := input.Read(buf)
			if err != nil || n == 0 {
				return
			}
			select {
			case <-ctx.Done():
				return
			case out <- buf[0]:
			}
		}
	}()
	return out
}

func parseByte(reader *byteReader, b byte) Event {
	if b == byteEscape {
		return readEscapeSequence(reader)
	}
	if b&0x80 != 0 {
		return readUTF8Sequence(reader, b)
	}
	return parseSingleByte(b)
}

func readUTF8Sequence(reader *byteReader, first byte) Event {
	n := utf8Bytes(first)
	if n == 1 {
		r, _ := utf8.DecodeRune([]byte{first})
		return Event{Type: KeyRune, Ch: r}
	}
	buf := make([]byte, n)
	buf[0] = first
	for i := 1; i < n; i++ {
		b, ok := reader.readByteTimeout(50 * time.Millisecond)
		if !ok {
			break
		}
		buf[i] = b
	}
	r, _ := utf8.DecodeRune(buf)
	return Event{Type: KeyRune, Ch: r}
}

func utf8Bytes(first byte) int {
	if first&0x80 == 0 {
		return 1
	}
	if first&0xE0 == 0xC0 {
		return 2
	}
	if first&0xF0 == 0xE0 {
		return 3
	}
	if first&0xF8 == 0xF0 {
		return 4
	}
	return 1
}

func readEscapeSequence(reader *byteReader) Event {
	// We already read ESC. Read the next byte with timeout.
	next, ok := reader.readByteTimeout(50 * time.Millisecond)
	if !ok {
		return Event{Type: KeyEscape}
	}

	if next == byteBracket {
		// CSI sequence: ESC [ ... final_byte
		return readCSI(reader)
	}

	if next == byteEscape {
		ev := readEscapeSequence(reader)
		ev.Alt = true
		return ev
	}

	if next == byteO {
		// SS3 sequence: ESC O X
		next2, ok2 := reader.readByteTimeout(50 * time.Millisecond)
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

	if next&0x80 != 0 {
		ev := readUTF8Sequence(reader, next)
		ev.Alt = true
		return ev
	}

	return Event{Type: KeyUnknown}
}

func readCSI(reader *byteReader) Event {
	// Read parameters and final byte
	params := make([]byte, 0, 8)
	final := byte(0)

	for {
		b, ok := reader.readByteTimeout(50 * time.Millisecond)
		if !ok {
			break
		}
		if b >= 0x40 && b <= 0x7E {
			final = b
			break
		}
		params = append(params, b)
	}

	if final == 'M' && len(params) == 0 {
		return readX10Mouse(reader)
	}
	return parseCSI(reader, params, final)
}

func readX10Mouse(reader *byteReader) Event {
	button, ok := reader.readByteTimeout(50 * time.Millisecond)
	if !ok {
		return Event{Type: KeyUnknown}
	}
	xb, ok := reader.readByteTimeout(50 * time.Millisecond)
	if !ok {
		return Event{Type: KeyUnknown}
	}
	yb, ok := reader.readByteTimeout(50 * time.Millisecond)
	if !ok {
		return Event{Type: KeyUnknown}
	}

	return parseMouseButton(int(button)-32, int(xb)-33, int(yb)-33, true)
}

func parseCSI(reader *byteReader, params []byte, final byte) Event {
	if len(params) > 0 && params[0] == '<' {
		return parseSGRMouse(params[1:], final)
	}

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
	if (final == 'M' || final == 'm') && len(paramNums) >= 3 {
		return parseMouseButton(paramNums[0], paramNums[1]-1, paramNums[2]-1, final == 'M')
	}

	modifier := 1 // default: no modifier
	if len(paramNums) >= 2 {
		modifier = paramNums[len(paramNums)-1]
	}
	if len(paramNums) == 0 {
		paramNums = append(paramNums, 1)
	}

	shift, ctrl, alt, cmd := modifierFlags(modifier)

	if final == '~' && len(paramNums) >= 3 && paramNums[0] == byteEscape {
		// xterm modifyOtherKeys protocol: ESC [ 27 ; modifier ; codepoint ~.
		shift, ctrl, alt, cmd = modifierFlags(paramNums[1])
		return modifiedCodepointEvent(paramNums[2], shift, ctrl, alt, cmd)
	}

	switch final {
	case 'A':
		return Event{Type: KeyArrowUp, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'B':
		return Event{Type: KeyArrowDown, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'C':
		return Event{Type: KeyArrowRight, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'D':
		return Event{Type: KeyArrowLeft, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'H':
		return Event{Type: KeyHome, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'F':
		return Event{Type: KeyEnd, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'Z':
		return Event{Type: KeyTab, Shift: true, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case 'u':
		// CSI u keyboard protocol, used by terminals that distinguish
		// modified keys from plain keys: ESC [ codepoint ; modifier u.
		return modifiedCodepointEvent(paramNums[0], shift, ctrl, alt, cmd)
	case '~':
		code := paramNums[0]
		switch code {
		case 1:
			return Event{Type: KeyHome, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
		case 3:
			return Event{Type: KeyDelete, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
		case 4:
			return Event{Type: KeyEnd, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
		case 5:
			return Event{Type: KeyPageUp, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
		case 6:
			return Event{Type: KeyPageDown, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
		case 200:
			return readBracketedPaste(reader)
		case 201:
			return Event{Type: KeyUnknown}
		}
	}

	return Event{Type: KeyUnknown}
}

func readBracketedPaste(reader *byteReader) Event {
	buf := make([]byte, 0, 256)
	for {
		b, ok := reader.readByteTimeout(50 * time.Millisecond)
		if !ok {
			return Event{Type: KeyPaste, Text: string(buf)}
		}
		buf = append(buf, b)
		if bytes.HasSuffix(buf, bracketedPasteEnd) {
			buf = buf[:len(buf)-len(bracketedPasteEnd)]
			return Event{Type: KeyPaste, Text: string(buf)}
		}
	}
}

func modifierFlags(modifier int) (shift, ctrl, alt, cmd bool) {
	bits := modifier - 1
	if bits < 0 {
		bits = 0
	}
	shift = bits&1 != 0
	alt = bits&2 != 0
	ctrl = bits&4 != 0
	cmd = bits&8 != 0
	return shift, ctrl, alt, cmd
}

func modifiedCodepointEvent(codepoint int, shift, ctrl, alt, cmd bool) Event {
	switch codepoint {
	case byteTab:
		return Event{Type: KeyTab, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case byteEnter:
		return Event{Type: KeyEnter, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	case byteEscape:
		return Event{Type: KeyEscape, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
	}

	ch := rune(codepoint)
	if (ctrl || cmd) && (ch == 'c' || ch == 'C') {
		return Event{Type: KeyCtrlC, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd, Ch: ch}
	}
	return Event{Type: KeyRune, Ch: ch, Shift: shift, Ctrl: ctrl, Alt: alt, Cmd: cmd}
}

func parseSGRMouse(params []byte, final byte) Event {
	var nums []int
	current := 0
	for _, b := range params {
		if b == byteSemicolon {
			nums = append(nums, current)
			current = 0
			continue
		}
		if b >= '0' && b <= '9' {
			current = current*10 + int(b-'0')
		}
	}
	nums = append(nums, current)
	if len(nums) < 3 {
		return Event{Type: KeyUnknown}
	}

	return parseMouseButton(nums[0], nums[1]-1, nums[2]-1, final == 'M')
}

func parseMouseButton(button, x, y int, isPress bool) Event {
	// Wheel events (button has bit 6 set) – only handle press ('M'); ignore release ('m').
	if isPress && button&64 != 0 {
		scrollButton := button & 3
		if scrollButton == 0 {
			return Event{Type: KeyMouseWheelUp, Mouse: true, MouseX: x, MouseY: y}
		}
		if scrollButton == 1 {
			return Event{Type: KeyMouseWheelDown, Mouse: true, MouseX: x, MouseY: y}
		}
		return Event{Type: KeyUnknown}
	}

	// Motion with no buttons (button == 3 or 35)
	if button == 3 || button == 35 {
		return Event{Type: KeyUnknown}
	}

	// Wheel release events have bit 6 set and final 'm' – ignore them.
	if !isPress && button&64 != 0 {
		return Event{Type: KeyUnknown}
	}

	// Drag events have bit 5 set (button + 32)
	isDrag := button&32 != 0
	baseButton := button & 3

	if isDrag {
		if baseButton == 0 {
			return Event{Type: KeyMouseLeftDrag, Mouse: true, MouseX: x, MouseY: y}
		}
		return Event{Type: KeyUnknown}
	}

	if isPress {
		if baseButton == 0 {
			return Event{Type: KeyMouseLeftDown, Mouse: true, MouseX: x, MouseY: y}
		}
		return Event{Type: KeyUnknown}
	}

	if !isPress {
		// SGR releases always report button 3, so we treat any release as left-up
		return Event{Type: KeyMouseLeftUp, Mouse: true, MouseX: x, MouseY: y}
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
