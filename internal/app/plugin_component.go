package app

import (
	"fmt"

	"github.com/Ceinl/plums/capabilities"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

type ComponentFactoryProvider interface {
	AppComponentFactory() ComponentFactory
}

type pluginComponent struct {
	name    string
	factory ComponentFactory
}

func NewPluginComponent(name string, factory ComponentFactory) capabilities.Component {
	return pluginComponent{name: name, factory: factory}
}

func (c pluginComponent) Name() string {
	return c.name
}

func (c pluginComponent) Arrange(capabilities.Rect) {}

func (c pluginComponent) Render(capabilities.RenderCtx, capabilities.Surface) {}

func (c pluginComponent) AppComponentFactory() ComponentFactory {
	return c.factory
}

func DefaultPluginComponents() []capabilities.Component {
	factories := DefaultComponentFactories()

	// Real public components: each renders via Surface from RenderCtx and owns
	// its ephemeral render state (no privileged *State factory path).
	components := []capabilities.Component{
		NewChatOutputComponent(),
		NewStatusBarComponent(),
		NewSplitStatusBarComponent(),
		NewStatusSeparatorComponent(),
		NewVerticalStatusSeparatorComponent(),
		NewInfoTabsComponent(),
		NewInfoViewComponent(),
		NewDiffLogComponent(),
		NewSessionsComponent(),
		NewSessionsVerticalComponent(),
		NewSessionsHorizontalComponent(),
		NewCommandPalettePanelComponent(),
	}

	// Phase 2b / Phase 3: editor (+ palette variant), input box, and the
	// fullscreen view still flow through the privileged factory path. The editor
	// buffer is owned by *State and its key input is routed centrally; porting
	// only its render surface lands in 2b. fullscreen_view is cut once the
	// fullscreen layout is removed in Phase 3.
	// TODO(phase2b/phase3): port editor/editor_or_palette/input_box/text_box and
	// cut fullscreen_view.
	names := []string{
		"editor",
		"editor_or_palette",
		"input_box",
		"text_box",
		"fullscreen_view",
	}
	for _, name := range names {
		factory := factories[name]
		if factory == nil {
			factory = missingPluginComponentFactory(name)
		}
		components = append(components, NewPluginComponent(name, factory))
	}
	return components
}

func missingPluginComponentFactory(name string) ComponentFactory {
	return func(*State, LayoutNode) (layout.Component, error) {
		return nil, fmt.Errorf("missing component factory %q", name)
	}
}
