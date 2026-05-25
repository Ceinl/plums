package app

import "testing"

func TestClampOutputScroll(t *testing.T) {
	state := NewState(80, 24)
	state.ScrollOutput(100)
	state.ClampOutputScroll(10)
	if got := state.OutputScroll(); got != 10 {
		t.Fatalf("expected scroll offset 10, got %d", got)
	}

	state.ClampOutputScroll(-1)
	if got := state.OutputScroll(); got != 0 {
		t.Fatalf("expected scroll offset 0, got %d", got)
	}
}

func TestMaxOutputScrollTracksViewport(t *testing.T) {
	state := NewState(100, 30)
	state.AddMessage("ai", "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten")

	state.Resize(100, 12)
	small := maxOutputScroll(state)
	if small == 0 {
		t.Fatal("expected small viewport to be scrollable")
	}

	state.Resize(100, 40)
	large := maxOutputScroll(state)
	if large >= small {
		t.Fatalf("expected larger viewport to reduce max scroll, got small=%d large=%d", small, large)
	}
}
