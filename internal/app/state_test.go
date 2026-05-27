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

func TestScrollOutputVisibleUsesRenderedMax(t *testing.T) {
	state := NewState(80, 24)
	state.SetOutputMaxScroll(10)

	state.ScrollOutputVisible(100)
	if got := state.OutputScroll(); got != 10 {
		t.Fatalf("expected scroll offset 10, got %d", got)
	}

	state.AddMessage("ai", "new content invalidates cached max")
	state.ScrollOutputVisible(5)
	if got := state.OutputScroll(); got != 15 {
		t.Fatalf("expected unclamped scroll while max is invalid, got %d", got)
	}
}

func TestLoadRenderConfig(t *testing.T) {
	if _, err := LoadRenderConfig(""); err != nil {
		t.Fatalf("load built-in render config: %v", err)
	}
	if _, err := LoadRenderConfig("../../docs/config/layout.json"); err != nil {
		t.Fatalf("load docs render config: %v", err)
	}
}
