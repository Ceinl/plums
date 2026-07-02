package app

import (
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// captureCtx is a minimal capabilities.Ctx that records Copy calls so the selection
// path can be asserted without shelling out to a real clipboard command.
type captureCtx struct {
	capabilities.Ctx
	copied string
}

func (c *captureCtx) Copy(text string) { c.copied = text }

func TestChatOutputComponentInstancesAreIndependent(t *testing.T) {
	template := NewChatOutputComponent()
	a, aok := template.(capabilities.ComponentInstancer)
	if !aok {
		t.Fatal("chat_output must be a ComponentInstancer")
	}
	first := a.NewComponent()
	second := a.NewComponent()
	if first == second {
		t.Fatal("NewComponent must return distinct instances")
	}
	if first.Name() != "chat_output" || second.Name() != "chat_output" {
		t.Fatalf("unexpected names %q %q", first.Name(), second.Name())
	}
}

func TestChatOutputComponentRendersThroughSurface(t *testing.T) {
	state := NewState(80, 24)
	state.AddMessage("user", "hello world")
	state.AddMessage("system", "a reply")

	component := NewChatOutputComponent().(capabilities.ComponentInstancer).NewComponent()
	component.Arrange(capabilities.Rect{X: 0, Y: 0, W: 80, H: 24})
	// Renders only through the public Surface; a panic here means the public
	// render contract is insufficient for the hardest component.
	component.Render(renderCtx{state: state, rect: capabilities.Rect{X: 0, Y: 0, W: 80, H: 24}}, screen.NewScreen(80, 24))
}

func TestChatOutputComponentOwnsScroll(t *testing.T) {
	component := NewChatOutputComponent().(capabilities.ComponentInstancer).NewComponent()
	scroller, ok := component.(capabilities.Scrollable)
	if !ok {
		t.Fatal("chat_output must be Scrollable")
	}
	// No content yet: nothing to scroll.
	if scroller.Scroll(3) {
		t.Fatal("expected no scroll change on empty body")
	}
	if scroller.ScrollToBottom() {
		t.Fatal("expected no change scrolling to bottom when already at bottom")
	}
}

func TestChatOutputComponentSelectionCopies(t *testing.T) {
	state := NewState(80, 24)
	for i := 0; i < 5; i++ {
		state.AddMessage("user", "selectable line of text")
	}
	component := NewChatOutputComponent().(capabilities.ComponentInstancer).NewComponent()
	component.Arrange(capabilities.Rect{X: 0, Y: 0, W: 80, H: 24})
	component.Render(renderCtx{state: state, rect: capabilities.Rect{X: 0, Y: 0, W: 80, H: 24}}, screen.NewScreen(80, 24))

	handler, ok := component.(capabilities.MouseHandler)
	if !ok {
		t.Fatal("chat_output must be a MouseHandler")
	}
	ctx := &captureCtx{}
	handler.HandleMouse(ctx, capabilities.MouseEvent{X: 2, Y: 0, Button: capabilities.MouseButtonLeft, Action: capabilities.MousePress})
	handler.HandleMouse(ctx, capabilities.MouseEvent{X: 18, Y: 0, Button: capabilities.MouseButtonLeft, Action: capabilities.MouseDrag})
	handler.HandleMouse(ctx, capabilities.MouseEvent{X: 18, Y: 0, Button: capabilities.MouseButtonLeft, Action: capabilities.MouseRelease})

	if provider, ok := component.(capabilities.SelectionProvider); !ok || provider.Selection() == "" {
		t.Fatalf("expected a non-empty selection after drag, got %q", ctxSelection(component))
	}
	if ctx.copied == "" {
		t.Fatal("expected selection to be copied via Ctx.Copy on release")
	}
}

func TestChatOutputComponentPaintsSlotBackground(t *testing.T) {
	state := NewState(80, 24)
	state.AddMessage("user", "hi")

	component := NewChatOutputComponent().(capabilities.ComponentInstancer).NewComponent()
	component.Arrange(capabilities.Rect{X: 0, Y: 0, W: 80, H: 24})
	scr := screen.NewScreen(80, 24)
	// A themed slot background (not empty -> not default black).
	component.Render(renderCtx{state: state, rect: capabilities.Rect{X: 0, Y: 0, W: 80, H: 24}, background: "themed-bg"}, scr)

	// An empty row well below the single message must be painted with the slot
	// background, not left as the default (black) background.
	if got := scr.Cell(5, 15).Bg; got != "themed-bg" {
		t.Fatalf("empty chat row background = %q, want the themed slot background", got)
	}
}

func TestPublicComponentInstancesKeyedBySlot(t *testing.T) {
	state := NewState(80, 24)
	factory := ComponentFactoryForPublic(NewChatOutputComponent())

	slotA, err := factory(state, LayoutNode{slotID: "/0"})
	if err != nil {
		t.Fatalf("factory slot A: %v", err)
	}
	slotB, err := factory(state, LayoutNode{slotID: "/1"})
	if err != nil {
		t.Fatalf("factory slot B: %v", err)
	}
	slotAAgain, err := factory(state, LayoutNode{slotID: "/0"})
	if err != nil {
		t.Fatalf("factory slot A rebuild: %v", err)
	}

	instA := slotA.(*publicComponentAdapter).component
	instB := slotB.(*publicComponentAdapter).component
	instAAgain := slotAAgain.(*publicComponentAdapter).component

	if instA == instB {
		t.Fatal("the same component in two slots must get independent instances")
	}
	if instA != instAAgain {
		t.Fatal("the same slot must reuse its cached instance across rebuilds")
	}
}

func ctxSelection(c capabilities.Component) string {
	if p, ok := c.(capabilities.SelectionProvider); ok {
		return p.Selection()
	}
	return ""
}
