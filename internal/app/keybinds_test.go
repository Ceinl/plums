package app

import (
	"context"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/keyboard"
)

func TestConfiguredKeybindRunsBuiltinAction(t *testing.T) {
	state := NewState(80, 24)
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'p', Ctrl: true}, []capabilities.Keybind{
		{Key: "ctrl+p", Do: "open_palette"},
	})

	if !handled {
		t.Fatal("expected configured keybind to be handled")
	}
	if !state.PopupOpen {
		t.Fatal("expected ctrl+p keybind to open palette")
	}
}

func TestConfiguredKeybindRunsSlashAction(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 's', Alt: true}, []capabilities.Keybind{
		{Key: "alt+s", Do: "/skills"},
	})

	if !handled {
		t.Fatal("expected configured keybind to be handled")
	}
	if got := runPending(state); !got.called("OpenSkills") {
		t.Fatalf("expected /skills keybind to call OpenSkills, got %v", got.calls)
	}
}

func TestConfiguredKeybindRunsNamedCommand(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'p', Ctrl: true}, []capabilities.Keybind{
		{Key: "ctrl+p", Do: "palette.open"},
	})

	if !handled {
		t.Fatal("expected configured keybind to be handled")
	}
	if got := runPending(state); !got.called("OpenCommandPalette") {
		t.Fatalf("expected palette.open keybind to call OpenCommandPalette, got %v", got.calls)
	}
}

func TestConfiguredKeybindQueuesRegistryCommand(t *testing.T) {
	state := NewState(80, 24)
	state.SetCommands([]capabilities.Command{
		{
			Name: "/custom",
			Do: func(context.Context, capabilities.Ctx) error {
				return nil
			},
		},
	})
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'k', Cmd: true}, []capabilities.Keybind{
		{Key: "cmd+k", Do: "/custom"},
	})

	if !handled {
		t.Fatal("expected configured keybind to be handled")
	}
	command, ok := state.ConsumePendingCommand()
	if !ok {
		t.Fatal("expected pending registry command")
	}
	if command.Name != "/custom" {
		t.Fatalf("pending command = %q, want /custom", command.Name)
	}
}

func TestConfiguredKeybindMatchesSpecialKey(t *testing.T) {
	state := NewState(80, 24)
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyEnter, Shift: true}, []capabilities.Keybind{
		{Key: "shift+enter", Do: "open_palette"},
	})

	if !handled {
		t.Fatal("expected configured keybind to be handled")
	}
	if !state.PopupOpen {
		t.Fatal("expected shift+enter keybind to open palette")
	}
}

func TestConfiguredKeybindOverridesLegacyDefault(t *testing.T) {
	state := stateWithBuiltinCommands(80, 24)
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'p', Ctrl: true}, []capabilities.Keybind{
		{Key: "ctrl+p", Do: "/skills"},
	})

	if !handled {
		t.Fatal("expected configured keybind to be handled")
	}
	if state.PopupOpen {
		t.Fatal("expected configured ctrl+p to avoid legacy palette toggle")
	}
	if got := runPending(state); !got.called("OpenSkills") {
		t.Fatalf("expected /skills keybind to call OpenSkills, got %v", got.calls)
	}
}

func TestConfiguredKeybindRequiresExactModifiers(t *testing.T) {
	state := NewState(80, 24)
	handled := HandleConfiguredKeybind(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'x', Ctrl: true}, []capabilities.Keybind{
		{Key: "alt+x", Do: "open_palette"},
	})

	if handled {
		t.Fatal("expected non-matching modifiers to be ignored")
	}
}
