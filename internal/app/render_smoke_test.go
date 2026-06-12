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
	cfg, err := LoadRenderConfig("")
	if err != nil {
		t.Fatalf("load render config: %v", err)
	}

	layouts := []LayoutType{LayoutChat, LayoutSplit, LayoutFullscreen}
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
		"fullscreen":      CreateFullscreenLayout,
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
