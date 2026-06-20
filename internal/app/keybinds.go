package app

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/keyboard"
)

type keybindSpec struct {
	keyType keyboard.EventType
	ch      rune
	shift   bool
	ctrl    bool
	alt     bool
	cmd     bool
}

// HandleConfiguredKeybind overlays user-owned keybindings on top of the legacy
// defaults. A matching key is consumed even if the configured action is invalid,
// so an overridden default does not leak through.
func HandleConfiguredKeybind(state *State, ev keyboard.Event, keybinds []capabilities.Keybind) bool {
	if len(keybinds) == 0 {
		return false
	}
	ev = normalizeKeybindEvent(ev)
	for _, keybind := range keybinds {
		spec, ok := parseKeybindSpec(keybind.Key)
		if !ok || !spec.matches(ev) {
			continue
		}
		if !state.RunKeybindAction(keybind.Do) {
			state.AddMessage("system", fmt.Sprintf("unknown keybind action %q", keybind.Do))
		}
		return true
	}
	return false
}

func (s *State) RunKeybindAction(action string) bool {
	action = strings.TrimSpace(action)
	if action == "" {
		return false
	}
	if strings.HasPrefix(action, "/") {
		return s.runSlashCommand(strings.TrimPrefix(action, "/"))
	}
	if builtin := s.commandConfig.actionFor(action); builtin != PaletteActionNone {
		if builtin == PaletteActionOpenPalette {
			s.OpenPalette()
			return true
		}
		s.PendingAction = builtin
		return true
	}
	return s.runSlashCommand(action)
}

func parseKeybindSpec(key string) (keybindSpec, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, " ", "")
	if key == "" {
		return keybindSpec{}, false
	}
	parts := strings.Split(key, "+")
	spec := keybindSpec{}
	keyPart := ""
	for _, part := range parts {
		if part == "" {
			return keybindSpec{}, false
		}
		switch part {
		case "shift":
			spec.shift = true
		case "ctrl", "control", "ctl", "^":
			spec.ctrl = true
		case "alt", "option", "opt", "meta":
			spec.alt = true
		case "cmd", "command", "super":
			spec.cmd = true
		default:
			if keyPart != "" {
				return keybindSpec{}, false
			}
			keyPart = part
		}
	}
	if keyPart == "" {
		return keybindSpec{}, false
	}
	keyType, ch, ok := parseKeybindKey(keyPart)
	if !ok {
		return keybindSpec{}, false
	}
	spec.keyType = keyType
	spec.ch = ch
	return spec, true
}

func parseKeybindKey(key string) (keyboard.EventType, rune, bool) {
	switch key {
	case "enter", "return":
		return keyboard.KeyEnter, 0, true
	case "backspace", "bs":
		return keyboard.KeyBackspace, 0, true
	case "tab":
		return keyboard.KeyTab, 0, true
	case "escape", "esc":
		return keyboard.KeyEscape, 0, true
	case "up", "arrowup", "arrow-up":
		return keyboard.KeyArrowUp, 0, true
	case "down", "arrowdown", "arrow-down":
		return keyboard.KeyArrowDown, 0, true
	case "right", "arrowright", "arrow-right":
		return keyboard.KeyArrowRight, 0, true
	case "left", "arrowleft", "arrow-left":
		return keyboard.KeyArrowLeft, 0, true
	case "home":
		return keyboard.KeyHome, 0, true
	case "end":
		return keyboard.KeyEnd, 0, true
	case "pageup", "page-up", "pgup":
		return keyboard.KeyPageUp, 0, true
	case "pagedown", "page-down", "pgdown", "pgdn":
		return keyboard.KeyPageDown, 0, true
	case "delete", "del":
		return keyboard.KeyDelete, 0, true
	case "space":
		return keyboard.KeyRune, ' ', true
	}
	ch, size := utf8.DecodeRuneInString(key)
	if ch == utf8.RuneError || size != len(key) {
		return keyboard.KeyUnknown, 0, false
	}
	return keyboard.KeyRune, ch, true
}

func (s keybindSpec) matches(ev keyboard.Event) bool {
	if ev.Type != s.keyType {
		return false
	}
	if ev.Shift != s.shift || ev.Ctrl != s.ctrl || ev.Alt != s.alt || ev.Cmd != s.cmd {
		return false
	}
	if s.keyType != keyboard.KeyRune {
		return true
	}
	return unicode.ToLower(ev.Ch) == unicode.ToLower(s.ch)
}

func normalizeKeybindEvent(ev keyboard.Event) keyboard.Event {
	ev = normalizeKeyEvent(ev)
	if ev.Type == keyboard.KeyCtrlC {
		ev.Type = keyboard.KeyRune
		ev.Ch = 'c'
		if !ev.Ctrl && !ev.Cmd {
			ev.Ctrl = true
		}
	}
	return ev
}
