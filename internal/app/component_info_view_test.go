package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

func TestInfoViewChatUsesRenderCtxBackground(t *testing.T) {
	state := NewState(32, 8)
	component := NewInfoViewComponent()
	rect := capabilities.Rect{X: 0, Y: 0, W: 32, H: 8}
	component.Arrange(rect)
	bg := "\x1b[48;2;9;8;7m"

	scr := screen.NewScreen(rect.W, rect.H)
	scr.SetOutput(nil)
	component.Render(renderCtx{state: state, rect: rect, background: bg}, scr)

	if got := scr.Cell(0, 0).Bg; got != bg {
		t.Fatalf("info_view bg = %q, want RenderCtx background %q", got, bg)
	}
}

func TestInfoViewGitDiffUsesRenderCtxBackground(t *testing.T) {
	state := NewState(32, 8)
	state.InfoView = InfoViewGitDiff
	state.GitDiff = "diff --git a/a b/a\n@@ -1 +1 @@\n-old\n+new\n"
	component := NewInfoViewComponent()
	rect := capabilities.Rect{X: 0, Y: 0, W: 32, H: 8}
	component.Arrange(rect)
	bg := "\x1b[48;2;8;7;6m"

	scr := screen.NewScreen(rect.W, rect.H)
	scr.SetOutput(nil)
	component.Render(renderCtx{state: state, rect: rect, background: bg}, scr)

	if got := scr.Cell(0, 0).Bg; got != bg {
		t.Fatalf("info_view git diff bg = %q, want RenderCtx background %q", got, bg)
	}
}

func TestSplitInfoViewWheelScrollsOutputTabs(t *testing.T) {
	silenceStdout(t)
	cfg := testRenderConfig(t)

	for _, tc := range []struct {
		name string
		view InfoView
	}{
		{name: "ai output", view: InfoViewAI},
		{name: "git diff", view: InfoViewGitDiff},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := NewState(120, 30)
			state.Layout = LayoutSplit
			state.InfoView = tc.view
			Render(state, cfg)

			adapter := publicComponentByName(state, "info_view")
			if adapter == nil {
				t.Fatal("split layout did not render info_view")
			}
			state.SetOutputMaxScroll(20)

			handled := HandlePublicComponentEvent(state, keyboard.Event{
				Type:   keyboard.KeyMouseWheelUp,
				Mouse:  true,
				MouseX: adapter.rect.X,
				MouseY: adapter.rect.Y,
			}, nil)
			if !handled {
				t.Fatal("wheel over split info_view was not handled")
			}
			if got := state.OutputScroll(); got != 3 {
				t.Fatalf("output scroll offset = %d, want 3", got)
			}
		})
	}
}

func TestSplitInfoViewIsActiveOutputScrollable(t *testing.T) {
	silenceStdout(t)
	cfg := testRenderConfig(t)
	state := NewState(120, 30)
	state.Layout = LayoutSplit
	Render(state, cfg)
	state.SetOutputMaxScroll(20)

	if !state.ScrollOutputVisible(3) {
		t.Fatal("split info_view did not handle visible output scroll")
	}
	if got := state.OutputScroll(); got != 3 {
		t.Fatalf("output scroll offset = %d, want 3", got)
	}
}

func publicComponentByName(state *State, name string) *publicComponentAdapter {
	for _, adapter := range state.publicComponents {
		if adapter.component != nil && adapter.component.Name() == name {
			return adapter
		}
	}
	return nil
}
