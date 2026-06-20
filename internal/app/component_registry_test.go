package app

import (
	"strings"
	"testing"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/keyboard"
	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

func TestDefaultComponentFactoriesResolveComponent(t *testing.T) {
	state := NewState(80, 24)
	component, err := buildComponent(state, LayoutNode{Component: "status_bar"})
	if err != nil {
		t.Fatalf("buildComponent() error = %v", err)
	}
	if _, ok := component.(*components.StatusBar); !ok {
		t.Fatalf("component type = %T, want *components.StatusBar", component)
	}
}

func TestBuildComponentRejectsUnknownName(t *testing.T) {
	state := NewState(80, 24)
	_, err := buildComponent(state, LayoutNode{Component: "missing"})
	if err == nil || !strings.Contains(err.Error(), "unknown component") {
		t.Fatalf("buildComponent() error = %v, want unknown component", err)
	}
}

func TestComponentFactoryOverride(t *testing.T) {
	state := NewState(80, 24)
	state.SetComponentFactories(map[string]ComponentFactory{
		"status_bar": func(*State, LayoutNode) (layout.Component, error) {
			text := components.NewText()
			text.SetContent("override")
			return text, nil
		},
	})

	component, err := buildComponent(state, LayoutNode{Component: "status_bar"})
	if err != nil {
		t.Fatalf("buildComponent() error = %v", err)
	}
	if _, ok := component.(*components.Text); !ok {
		t.Fatalf("component type = %T, want *components.Text", component)
	}
}

type publicComponentRecorder struct {
	name      string
	arranged  capabilities.Rect
	renderCtx capabilities.RenderCtx
	surface   capabilities.Surface
}

func (c *publicComponentRecorder) Name() string {
	return c.name
}

func (c *publicComponentRecorder) Arrange(rect capabilities.Rect) {
	c.arranged = rect
}

func (c *publicComponentRecorder) Render(ctx capabilities.RenderCtx, surface capabilities.Surface) {
	c.renderCtx = ctx
	c.surface = surface
}

type publicCapabilityRecorder struct {
	publicComponentRecorder
	dirty        bool
	clearedDirty bool
	selection    string
	key          capabilities.KeyEvent
	mouse        capabilities.MouseEvent
	keyHandled   bool
	mouseHandled bool
}

func (c *publicCapabilityRecorder) IsDirty() bool {
	return c.dirty
}

func (c *publicCapabilityRecorder) ClearDirty() {
	c.dirty = false
	c.clearedDirty = true
}

func (c *publicCapabilityRecorder) Selection() string {
	return c.selection
}

func (c *publicCapabilityRecorder) HandleKey(ctx capabilities.Ctx, event capabilities.KeyEvent) bool {
	c.key = event
	return c.keyHandled
}

func (c *publicCapabilityRecorder) HandleMouse(ctx capabilities.Ctx, event capabilities.MouseEvent) bool {
	c.mouse = event
	return c.mouseHandled
}

func TestPublicComponentAdapterReceivesRenderContext(t *testing.T) {
	state := NewState(80, 24)
	state.AddMessage("user", "hello")
	state.SetSessionID("s1")
	state.SetSessionTitle("Session")
	state.SetModel("provider", "model")
	state.SetTheme(capabilities.Theme{Name: "zen"})
	state.SetStreaming(true)
	state.Editor.SetContent("draft")

	recorder := &publicComponentRecorder{name: "custom"}
	factory := ComponentFactoryForPublic(recorder)
	component, err := factory(state, LayoutNode{Component: "custom"})
	if err != nil {
		t.Fatalf("factory error = %v", err)
	}
	if len(state.publicComponents) != 1 {
		t.Fatalf("tracked public components = %d, want 1", len(state.publicComponents))
	}

	component.Layout(1, 2, 30, 4)
	component.Render(nil)

	if recorder.arranged != (capabilities.Rect{X: 1, Y: 2, W: 30, H: 4}) {
		t.Fatalf("arranged rect = %+v", recorder.arranged)
	}
	if recorder.renderCtx == nil {
		t.Fatal("public component did not receive RenderCtx")
	}
	if got := recorder.renderCtx.Rect(); got != recorder.arranged {
		t.Fatalf("ctx rect = %+v, want %+v", got, recorder.arranged)
	}
	messages := recorder.renderCtx.Messages()
	if len(messages) != 1 || messages[0].Role != "user" || messages[0].Content != "hello" {
		t.Fatalf("ctx messages = %+v", messages)
	}
	if session := recorder.renderCtx.Session(); session.ID != "s1" || session.Title != "Session" || session.Model == nil || session.Model.ID != "model" {
		t.Fatalf("ctx session = %+v", session)
	}
	if !recorder.renderCtx.Streaming() {
		t.Fatal("ctx streaming = false, want true")
	}
	if got := recorder.renderCtx.Theme().Name; got != "zen" {
		t.Fatalf("ctx theme = %q, want zen", got)
	}
	if got := recorder.renderCtx.Input().Text(); got != "draft" {
		t.Fatalf("ctx input = %q, want draft", got)
	}
}

func TestPublicComponentAdapterHonorsOptionalCapabilities(t *testing.T) {
	state := NewState(80, 24)
	recorder := &publicCapabilityRecorder{
		publicComponentRecorder: publicComponentRecorder{name: "custom"},
		dirty:                   true,
		selection:               "selected by component",
	}
	factory := ComponentFactoryForPublic(recorder)
	component, err := factory(state, LayoutNode{Component: "custom"})
	if err != nil {
		t.Fatalf("factory error = %v", err)
	}

	if !component.IsDirty() {
		t.Fatal("component IsDirty() = false, want public DirtyTracker result")
	}
	component.ClearDirty()
	if recorder.dirty || !recorder.clearedDirty {
		t.Fatalf("public dirty tracker not cleared: dirty=%v cleared=%v", recorder.dirty, recorder.clearedDirty)
	}
	if got := state.PublicComponentSelection(); got != "selected by component" {
		t.Fatalf("public component selection = %q", got)
	}
	ctx := newRuntimeCtx(state, RunConfig{}, nil, nil)
	if got := ctx.Selection(); got != "selected by component" {
		t.Fatalf("runtime ctx selection = %q", got)
	}
}

func TestPublicComponentKeyHandlerReceivesEvents(t *testing.T) {
	state := NewState(80, 24)
	recorder := &publicCapabilityRecorder{
		publicComponentRecorder: publicComponentRecorder{name: "custom"},
		keyHandled:              true,
	}
	factory := ComponentFactoryForPublic(recorder)
	if _, err := factory(state, LayoutNode{Component: "custom"}); err != nil {
		t.Fatalf("factory error = %v", err)
	}

	handled := HandlePublicComponentEvent(state, keyboard.Event{Type: keyboard.KeyRune, Ch: 'K', Cmd: true}, nil)
	if !handled {
		t.Fatal("public component key handler did not handle event")
	}
	if recorder.key.Key != "k" || recorder.key.Rune != 'K' || !recorder.key.Cmd {
		t.Fatalf("public key event = %+v, want cmd+K", recorder.key)
	}
}

func TestPublicComponentMouseHandlerTargetsArrangedRect(t *testing.T) {
	state := NewState(80, 24)
	recorder := &publicCapabilityRecorder{
		publicComponentRecorder: publicComponentRecorder{name: "custom"},
		mouseHandled:            true,
	}
	factory := ComponentFactoryForPublic(recorder)
	component, err := factory(state, LayoutNode{Component: "custom"})
	if err != nil {
		t.Fatalf("factory error = %v", err)
	}
	component.Layout(5, 6, 10, 3)

	outside := HandlePublicComponentEvent(state, keyboard.Event{Type: keyboard.KeyMouseLeftDown, Mouse: true, MouseX: 4, MouseY: 7}, nil)
	if outside {
		t.Fatal("mouse event outside component was handled")
	}
	inside := HandlePublicComponentEvent(state, keyboard.Event{Type: keyboard.KeyMouseLeftDown, Mouse: true, MouseX: 6, MouseY: 7}, nil)
	if !inside {
		t.Fatal("mouse event inside component was not handled")
	}
	if recorder.mouse.X != 6 || recorder.mouse.Y != 7 || recorder.mouse.Button != capabilities.MouseButtonLeft || recorder.mouse.Action != capabilities.MousePress {
		t.Fatalf("public mouse event = %+v", recorder.mouse)
	}
}
