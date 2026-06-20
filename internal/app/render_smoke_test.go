package app

import (
	"os"
	"testing"

	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// silenceStdout keeps the screen escape-code stream out of test output, since
// screen.NewScreen always writes to os.Stdout.
func silenceStdout(t *testing.T) {
	t.Helper()
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	orig := os.Stdout
	os.Stdout = devNull
	scr = nil // drop any screen already bound to the real stdout
	t.Cleanup(func() {
		os.Stdout = orig
		scr = nil
		devNull.Close()
	})
}

func smokeState(w, h int) *State {
	state := NewState(w, h)
	state.SetSessionItems([]SessionListItem{
		{ID: "s1", Title: "One", Current: true},
		{ID: "s2", Title: "Two"},
	})
	state.messages = []Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "# hi\n\nsome `code` and\n```go\nfmt.Println(\"x\")\n```\n- a list"},
	}
	state.GitDiff = "diff --git a/f b/f\n@@ -1 +1 @@\n-old\n+new\n"
	return state
}

// TestRenderAllLayoutsAndSizes renders every layout through the full Render
// pipeline at wide and narrow sizes, with and without the palette popup, to
// catch panics and layout regressions across all modules.
func TestRenderAllLayoutsAndSizes(t *testing.T) {
	silenceStdout(t)
	cfg := testRenderConfig(t)

	layouts := []LayoutType{LayoutChat, LayoutSplit, LayoutZen}
	sizes := [][2]int{{160, 48}, {120, 40}, {80, 24}, {40, 12}}
	for _, lt := range layouts {
		for _, size := range sizes {
			for _, popup := range []bool{false, true} {
				state := smokeState(size[0], size[1])
				state.Layout = lt
				state.PopupOpen = popup
				Render(state, cfg)
			}
		}
	}
}

// TestFallbackLayoutBuilders exercises the Go layout builders used when the
// JSON layout config cannot be loaded.
func TestFallbackLayoutBuilders(t *testing.T) {
	builders := map[string]func(*State) *components.Div{
		"chat":            CreateChatLayout,
		"split":           CreateSplitLayout,
		"sessions":        CreateSessionsLayout,
		"narrow_split":    CreateNarrowSplitLayout,
		"narrow_sessions": CreateNarrowSessionsLayout,
		"zen":             CreateZenLayout,
	}
	sizes := [][2]int{{120, 40}, {40, 12}}
	for name, build := range builders {
		for _, size := range sizes {
			state := smokeState(size[0], size[1])
			root := build(state)
			if root == nil {
				t.Fatalf("%s builder returned nil", name)
			}
			root.Layout(0, 0, size[0], size[1])
			root.Render(screen.NewScreen(size[0], size[1]))
		}
	}
}

// TestEmbeddedDefaultExposesZen guards against the embedded default config
// drifting out of sync with the Go fallbacks: the built-in layout JSON must
// itself list zen in the menu, not merely render it via CreateZenLayout. (The
// broader smoke test would pass on the Go fallback alone and hide a missing
// JSON entry.)
func TestEmbeddedDefaultExposesZen(t *testing.T) {
	cfg := testRenderConfig(t)
	if _, ok := cfg.Layouts["zen"]; !ok {
		t.Fatal("embedded default config is missing the 'zen' layout node")
	}
	found := false
	for _, lt := range cfg.AvailableLayoutTypes() {
		if lt == LayoutZen {
			found = true
		}
	}
	if !found {
		t.Fatalf("zen not selectable from embedded default; menu=%v", cfg.Menu)
	}
}

// TestCustomConfigLayoutNeedsNoGo proves the layout system is data-driven: a
// layout defined only in config (a new "layouts" entry plus a "menu" entry,
// with no Go LayoutType constant) becomes selectable, cycles in, and renders.
func TestCustomConfigLayoutNeedsNoGo(t *testing.T) {
	silenceStdout(t)
	cfg := testRenderConfig(t)

	// Define a bespoke single-column layout purely as data.
	cfg.Layouts["focus"] = LayoutNode{
		Type:      "div",
		Size:      SizeNode{Width: []byte(`"100%"`), Height: []byte(`"100%"`)},
		Direction: "column",
		Children: []LayoutNode{
			{Component: "chat_output", Size: SizeNode{Width: []byte(`"100%"`), Height: []byte(`"grow"`)}},
			{Component: "input_box", Size: SizeNode{Width: []byte(`"100%"`), Height: []byte(`7`)}},
		},
	}
	cfg.Menu = append(cfg.Menu, "focus")

	available := cfg.AvailableLayoutTypes()
	if available[len(available)-1] != LayoutType("focus") {
		t.Fatalf("expected custom 'focus' layout to be available, got %v", available)
	}

	state := smokeState(120, 40)
	state.SetAvailableLayouts(available)
	state.SetLayout(LayoutType("focus"))
	if state.LayoutLabel() != "focus" || layoutTitle(state.Layout) != "Focus" {
		t.Fatalf("unexpected label/title for custom layout: %q / %q", state.LayoutLabel(), layoutTitle(state.Layout))
	}
	Render(state, cfg) // must not panic and must use the config-defined tree
}
