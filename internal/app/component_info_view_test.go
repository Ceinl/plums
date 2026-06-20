package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
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
