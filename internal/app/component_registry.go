package app

import (
	"fmt"

	"github.com/Ceinl/plums/internal/ui/tui/components"
	"github.com/Ceinl/plums/internal/ui/tui/layout"
)

type ComponentFactory func(*State, LayoutNode) (layout.Component, error)

func DefaultComponentFactories() map[string]ComponentFactory {
	factories := map[string]ComponentFactory{
		"chat_output": func(state *State, node LayoutNode) (layout.Component, error) {
			return newChatLog(state), nil
		},
		"status_separator":          separatorFactory,
		"vertical_status_separator": separatorFactory,
		"editor": func(state *State, node LayoutNode) (layout.Component, error) {
			return state.Editor, nil
		},
		"editor_or_palette": func(state *State, node LayoutNode) (layout.Component, error) {
			return state.Editor, nil
		},
		"input_box": inputBoxFactory,
		"text_box":  inputBoxFactory,
		"command_palette_panel": func(state *State, node LayoutNode) (layout.Component, error) {
			popup := components.NewPopup()
			popup.SetPanel(true)
			popup.SetTitle(state.PaletteTitle())
			popup.SetQuery(state.PaletteSearch())
			popup.SetItems(state.PaletteItems(), state.PaletteIndex)
			return popup, nil
		},
		"info_tabs": func(state *State, node LayoutNode) (layout.Component, error) {
			tabs := components.NewInfoTabs()
			tabs.SetTabs([]components.InfoTab{
				{Label: "AI output", Active: state.InfoView == InfoViewAI},
				{Label: "Git diff", Active: state.InfoView == InfoViewGitDiff},
			})
			return tabs, nil
		},
		"sessions": func(state *State, node LayoutNode) (layout.Component, error) {
			return newSessions(state, components.SessionsVertical), nil
		},
		"sessions_vertical": func(state *State, node LayoutNode) (layout.Component, error) {
			return newSessions(state, components.SessionsVertical), nil
		},
		"sessions_horizontal": func(state *State, node LayoutNode) (layout.Component, error) {
			return newSessions(state, components.SessionsHorizontal), nil
		},
		"info_view": func(state *State, node LayoutNode) (layout.Component, error) {
			if state.InfoView == InfoViewGitDiff && node.Variants["git_diff"] == "git_diff_log" {
				return newGitDiffLog(state), nil
			}
			return newChatLog(state), nil
		},
		"fullscreen_view": func(state *State, node LayoutNode) (layout.Component, error) {
			return newFullscreenView(state), nil
		},
		"split_status_bar": statusBarFactory(true),
		"status_bar":       statusBarFactory(false),
	}
	return factories
}

func (s *State) SetComponentFactories(factories map[string]ComponentFactory) {
	s.componentFactories = cloneComponentFactories(factories)
}

func (s *State) componentFactory(name string) (ComponentFactory, bool) {
	if len(s.componentFactories) == 0 {
		s.componentFactories = DefaultComponentFactories()
	}
	factory, ok := s.componentFactories[name]
	return factory, ok
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

func separatorFactory(state *State, node LayoutNode) (layout.Component, error) {
	sep := components.NewSeparator()
	sep.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
	return sep, nil
}

func inputBoxFactory(state *State, node LayoutNode) (layout.Component, error) {
	box := components.NewInputBox(state.Editor)
	box.SetStatusSegments(chatStatusSegments(state))
	return box, nil
}

func statusBarFactory(showSession bool) ComponentFactory {
	return func(state *State, node LayoutNode) (layout.Component, error) {
		bar := components.NewStatusBar()
		bar.SetStatus(state.ServerStarting, state.ServerReady, state.IsStreaming())
		bar.SetSession(state.SessionTitle)
		bar.SetMode(state.Mode)
		bar.SetModel(state.ModelProvider, state.ModelID)
		bar.SetShowSession(showSession)
		return bar, nil
	}
}
