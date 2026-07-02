package app

import (
	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/components"
)

// chatOutputComponent is the public chat_output pane. It owns its ChatLog
// instance and all of its interaction state (selection, scroll), reading only
// message data from RenderCtx and drawing only through Surface — the reference
// dogfood of the public component API (no privileged *State path).
type chatOutputComponent struct {
	chatLog   *components.ChatLog
	rect      capabilities.Rect
	selection string
}

// NewChatOutputComponent returns the registry template. The kernel instantiates
// one per layout slot via NewComponent so each owns private interaction state.
func NewChatOutputComponent() capabilities.Component {
	return &chatOutputComponent{}
}

func (c *chatOutputComponent) Name() string { return "chat_output" }

func (c *chatOutputComponent) NewComponent() capabilities.Component {
	return &chatOutputComponent{chatLog: components.NewChatLog()}
}

func (c *chatOutputComponent) log() *components.ChatLog {
	if c.chatLog == nil {
		c.chatLog = components.NewChatLog()
	}
	return c.chatLog
}

func (c *chatOutputComponent) Arrange(rect capabilities.Rect) {
	c.rect = rect
}

func (c *chatOutputComponent) Render(rctx capabilities.RenderCtx, surface capabilities.Surface) {
	log := c.log()
	msgs := rctx.Messages()
	chatMessages := make([]components.ChatMessage, len(msgs))
	for i, m := range msgs {
		chatMessages[i] = components.ChatMessage{Role: m.Role, Content: m.Content}
	}
	log.SetMessages(chatMessages)
	log.SetStreaming(rctx.Streaming())
	if cv, ok := rctx.(capabilities.ChatView); ok {
		log.SetAiOutput(cv.StreamingText())
		log.SetThinkingVisibility(components.ThinkingVisibility(cv.ThinkingVisibility()))
		log.SetToolCallVisibility(components.ToolCallVisibility(cv.ToolCallVisibility()))
	}
	log.SetBackground(rctx.Background())
	log.Layout(c.rect.X, c.rect.Y, c.rect.W, c.rect.H)
	log.RenderSurface(surface)
}

func (c *chatOutputComponent) HandleMouse(ctx capabilities.Ctx, ev capabilities.MouseEvent) bool {
	if ev.Button != capabilities.MouseButtonLeft {
		return false
	}
	log := c.log()
	switch ev.Action {
	case capabilities.MousePress:
		return log.MouseDown(ev.X, ev.Y)
	case capabilities.MouseDrag:
		log.MouseDrag(ev.X, ev.Y)
		return true
	case capabilities.MouseRelease:
		if text := log.MouseUp(ev.X, ev.Y); text != "" {
			c.selection = text
			ctx.Copy(text)
		}
		return true
	}
	return false
}

func (c *chatOutputComponent) Selection() string { return c.selection }

func (c *chatOutputComponent) Scroll(delta int) bool { return c.log().Scroll(delta) }

func (c *chatOutputComponent) ScrollToBottom() bool { return c.log().ScrollToBottom() }
