package app

import (
	"fmt"

	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

// ComponentFactory builds a layout.Component for a layout slot. Every factory is
// produced by ComponentFactoryForPublic, wrapping a public capabilities.Component
// — there is no privileged *State factory path.
type ComponentFactory func(*State, LayoutNode) (layout.Component, error)

func (s *State) SetComponentFactories(factories map[string]ComponentFactory) {
	s.componentFactories = cloneComponentFactories(factories)
}

func (s *State) componentFactory(name string) (ComponentFactory, bool) {
	if len(s.componentFactories) == 0 {
		s.componentFactories = defaultComponentFactories()
	}
	factory, ok := s.componentFactories[name]
	return factory, ok
}

// defaultComponentFactories builds the built-in component factory map straight
// from the public DefaultComponents, each wrapped via ComponentFactoryForPublic.
// It is the same map the kernel produces from its registry; State falls back to
// it so a bare State (tests, doctor) can resolve the stock layouts.
func defaultComponentFactories() map[string]ComponentFactory {
	components := DefaultComponents()
	out := make(map[string]ComponentFactory, len(components))
	for _, component := range components {
		out[component.Name()] = ComponentFactoryForPublic(component)
	}
	return out
}

func cloneComponentFactories(factories map[string]ComponentFactory) map[string]ComponentFactory {
	if len(factories) == 0 {
		return nil
	}
	out := make(map[string]ComponentFactory, len(factories))
	for name, factory := range factories {
		if name == "" || factory == nil {
			continue
		}
		out[name] = factory
	}
	return out
}

func buildRegisteredComponent(state *State, node LayoutNode) (layout.Component, error) {
	factory, ok := state.componentFactory(node.Component)
	if !ok {
		return nil, fmt.Errorf("unknown component %q", node.Component)
	}
	return factory(state, node)
}
