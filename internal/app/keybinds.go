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

// RunKeybindAction resolves a keybind's Do string against the registered
// commands. Exact command names win first ("palette.open", "prompt.submit",
// "/new"). Legacy action keywords ("open_palette", "change_model", ...) map to
// the equivalent registered command or slash command so existing keymaps keep
// working. The bare "open_palette" still opens the palette directly when no
// command registry is installed, which keeps low-level state tests usable.
func (s *State) RunKeybindAction(action string) bool {
	action = strings.TrimSpace(action)
	if action == "" {
		return false
	}
	if s.queueCommandByName(action) {
		return true
	}
	if strings.HasPrefix(action, "/") {
		return s.runSlashCommand(strings.TrimPrefix(action, "/"))
	}
	if name, ok := keybindActionAliases[action]; ok {
		if s.queueCommandByName(name) {
			return true
		}
		if strings.HasPrefix(name, "/") {
			return s.runSlashCommand(strings.TrimPrefix(name, "/"))
		}
		if s.runSlashCommand(name) {
			return true
		}
		if action == "open_palette" {
			s.OpenPalette()
			return true
		}
		return false
	}
	return s.runSlashCommand(action)
}

// queueCommandByName queues an exact registered command name for the run loop.
// The command executes later through the same path as palette and slash command
// picks, keeping keybind handling free of feature-specific behavior.
func (s *State) queueCommandByName(name string) bool {
	for _, command := range s.commands {
		if command.Name != name || command.Do == nil {
			continue
		}
		s.pendingCommand = command.Name
		return true
	}
	return false
}

// keybindActionAliases maps the legacy action keywords (the old commands.json
// action names) to registered command names. These aliases are compatibility
// only; the Default Config uses the canonical command names directly.
var keybindActionAliases = map[string]string{
	"open_palette":  "palette.open",
	"new_session":   "/new",
	"change_model":  "/model",
	"backend_list":  "/backend",
	"skills_list":   "/skills",
	"sessions_list": "/sessions",
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
