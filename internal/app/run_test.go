package app

import (
	"testing"
	"time"

	"github.com/Ceinl/plums/internal/keyboard"
)

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
