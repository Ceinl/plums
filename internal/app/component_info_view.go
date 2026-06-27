package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/screen"
)

// infoViewComponent is the public info_view pane used by the split layout: it
// shows the chat log or the git diff log depending on the active info tab. Both
// bodies are the State-owned widgets, with input routed through this component.
type infoViewComponent struct {
	rect  capabilities.Rect
	state *State
}

var _ capabilities.Scrollable = (*infoViewComponent)(nil)

func NewInfoViewComponent() capabilities.Component {
	return &infoViewComponent{}
}

func (c *infoViewComponent) Name() string { return "info_view" }

func (c *infoViewComponent) Arrange(rect capabilities.Rect) { c.rect = rect }

func (c *infoViewComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	scr, ok := surface.(*screen.Screen)
	if !ok {
		return
	}
	provider, ok := rctx.(appStateProvider)
	if !ok {
		return
	}
	state := provider.appState()
	if state == nil {
		return
	}
	c.state = state

	if rctx.InfoView() == "git_diff" {
		diff := state.DiffLog()
		diff.SetContent(rctx.GitDiff())
		diff.SetScrollOffset(state.OutputScroll())
		diff.SetStyle(bgStyle(rctx.Background()))
		diff.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
		diff.Render(scr)
		return
	}

	renderStateChatLog(rctx, state, c.rect, scr)
}

func (c *infoViewComponent) HandleMouse(ctx capabilities.Ctx, ev capabilities.MouseEvent) bool {
	if c.state == nil || c.state.PopupOpen {
		return false
	}
	switch ev.Action {
	case capabilities.MousePress:
		return c.state.OutputMouseDown(ev.X, ev.Y)
	case capabilities.MouseDrag:
		return c.state.OutputMouseDrag(ev.X, ev.Y)
	case capabilities.MouseRelease:
		text := c.state.OutputMouseUp(ev.X, ev.Y)
		if text != "" && ctx != nil {
			ctx.Copy(text)
		}
		return true
	default:
		return false
	}
}

func (c *infoViewComponent) Scroll(delta int) bool {
	if c.state == nil {
		return false
	}
	return c.state.scrollOutputOffset(delta)
}

func (c *infoViewComponent) ScrollToBottom() bool {
	if c.state == nil {
		return false
	}
	return c.state.scrollOutputOffsetBottom()
}

// renderStateChatLog draws the State-owned chat log with the message/streaming
// data from RenderCtx and the central scroll offset, mirroring the legacy
// newChatLog factory.
func renderStateChatLog(rctx capabilities.RenderCtx, state *State, rect capabilities.Rect, scr *screen.Screen) {
	log := state.ChatLog()
	msgs := rctx.Messages()
	chatMessages := make([]components.ChatMessage, len(msgs))
	for i, m := range msgs {
		chatMessages[i] = components.ChatMessage{Role: m.Role, Content: m.Content}
	}
	log.SetMessages(chatMessages)
	log.SetAiOutput(rctx.StreamingText())
	log.SetStreaming(rctx.Streaming())
	log.SetThinkingVisibility(components.ThinkingVisibility(rctx.ThinkingVisibility()))
	log.SetToolCallVisibility(components.ToolCallVisibility(rctx.ToolCallVisibility()))
	log.SetScrollOffset(state.OutputScroll())
	log.SetBackground(rctx.Background())
	log.Layout(rect.X, rect.Y, rect.W, rect.H)
	log.RenderSurface(scr)
}
