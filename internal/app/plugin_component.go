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
	names := []string{
		"status_separator",
		"vertical_status_separator",
		"editor",
		"editor_or_palette",
		"input_box",
		"text_box",
		"command_palette_panel",
		"info_tabs",
		"sessions",
		"sessions_vertical",
		"sessions_horizontal",
		"info_view",
		"fullscreen_view",
		"split_status_bar",
		"status_bar",
	}
	// chat_output is a real public component (renders via Surface, owns its own
	// selection/scroll) rather than a factory-wrapped privileged component.
	components := []capabilities.Component{NewChatOutputComponent()}
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
