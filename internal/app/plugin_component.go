package app

import (
	"github.com/Ceinl/plums/capabilities"
)

// DefaultComponents returns the built-in public components. Every entry is a real
// capabilities.Component that renders through Surface from RenderCtx; there is no
// privileged *State factory path. State-entangled panes (editor, sessions,
// info_view, ...) render a State-owned widget via the in-package appStateProvider
// seam so central key/mouse/scroll routing keeps targeting the same instance.
func DefaultComponents() []capabilities.Component {
	return []capabilities.Component{
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
		NewEditorComponent(),
		NewEditorOrPaletteComponent(),
		NewInputBoxComponent(),
		NewTextBoxComponent(),
		NewFullscreenViewComponent(),
	}
}
