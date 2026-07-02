package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

func TestSplitModelRef(t *testing.T) {
	cases := []struct {
		ref             string
		provider, model string
		ok              bool
	}{
		{"", "", "", false},
		{"  ", "", "", false},
		{"anthropic/claude", "anthropic", "claude", true},
		{"claude", "", "claude", true},
		{" openai/gpt-5 ", "openai", "gpt-5", true},
	}
	for _, c := range cases {
		provider, model, ok := splitModelRef(c.ref)
		if provider != c.provider || model != c.model || ok != c.ok {
			t.Errorf("splitModelRef(%q) = (%q,%q,%v), want (%q,%q,%v)",
				c.ref, provider, model, ok, c.provider, c.model, c.ok)
		}
	}
}

func TestDoubleEscapeStopperStopsOnSecondEscapeWhileStreaming(t *testing.T) {
	var stopper doubleEscapeStopper
	now := time.Now()

	if stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now) {
		t.Fatalf("expected first escape to arm stop only")
	}
	if !stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now.Add(doubleEscapeStopWindow)) {
		t.Fatalf("expected second escape inside window to stop")
	}
}

func TestDoubleEscapeStopperRequiresStreaming(t *testing.T) {
	var stopper doubleEscapeStopper
	now := time.Now()

	if stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, false, now) {
		t.Fatalf("expected escape outside streaming to be ignored")
	}
	if stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now.Add(time.Millisecond)) {
		t.Fatalf("expected first streaming escape to arm stop only")
	}
}

func TestDoubleEscapeStopperResetsOnOtherKeyOrTimeout(t *testing.T) {
	var stopper doubleEscapeStopper
	now := time.Now()

	stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now)
	if stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyRune, Ch: 'x'}, true, now.Add(time.Millisecond)) {
		t.Fatalf("expected non-escape key to reset without stopping")
	}
	if stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now.Add(2*time.Millisecond)) {
		t.Fatalf("expected escape after reset to arm stop only")
	}

	stopper.Reset()
	stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now)
	if stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape}, true, now.Add(doubleEscapeStopWindow+time.Nanosecond)) {
		t.Fatalf("expected escape after timeout to arm stop only")
	}
}

func TestDoubleEscapeStopperTreatsAltEscapeAsDoubleEscape(t *testing.T) {
	var stopper doubleEscapeStopper

	if !stopper.ShouldStop(keyboard.Event{Type: keyboard.KeyEscape, Alt: true}, true, time.Now()) {
		t.Fatalf("expected alt escape event to stop while streaming")
	}
}

func TestCtrlPKeybindOpensCommandPaletteInRunLoop(t *testing.T) {
	oldScreen := scr
	t.Cleanup(func() { scr = oldScreen })
	scr = screen.NewScreen(80, 24)

	keys := make(chan keyboard.Event, 2)

	var out lockedWriter
	scr.SetOutput(&out)

	done := make(chan error, 1)
	go func() {
		_, err := Run(context.Background(), Deps{
			Terminal:     &ui.Terminal{W: 80, H: 24},
			Keyboard:     keys,
			RenderConfig: testRenderConfig(t),
			Layouts:      []LayoutType{LayoutZen},
			Commands:     builtinCommands(),
			Keybinds:     []capabilities.Keybind{{Key: "ctrl+p", Do: "palette.open"}},
			Components:   defaultComponentFactories(),
			Backends: []BackendRuntime{{
				ID:      "test",
				Name:    "test",
				Backend: backendRuntimeTestBackend{},
			}},
		}, RunConfig{
			BackendProvider: "test",
			DefaultLayout:   "zen",
		})
		done <- err
	}()

	keys <- keyboard.Event{Type: keyboard.KeyRune, Ch: 'P', Ctrl: true}
	deadline := time.After(500 * time.Millisecond)
	for !out.Contains("Command Palette") {
		select {
		case <-deadline:
			close(keys)
			if err := <-done; err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			t.Fatalf("expected ctrl+p to render command palette, output did not contain title: %q", out.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
	close(keys)
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

type lockedWriter struct {
	mu sync.Mutex
	b  strings.Builder
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *lockedWriter) Contains(s string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Contains(w.b.String(), s)
}

func (w *lockedWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}
